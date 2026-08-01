package scanner

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"runtime"
)

// ArchiveFile represents a compressed archive file
type ArchiveFile struct {
	Name      string
	Path      string
	Size      int64
	Type      string    // "zip", "rar", "7z"
	ModTime   time.Time // Modification time
	FileCount int       // Number of files inside
}

// IsMultiVolumePart returns true if the file looks like a part of a multi-volume archive.
// It returns (isPart, baseName, partSuffix)
func (f ArchiveFile) IsMultiVolumePart() (bool, string, string) {
	name := strings.ToLower(f.Name)

	// common separators for "partN"
	separators := []string{".part", "_part", "-part", " part"}
	for _, sep := range separators {
		if strings.Contains(name, sep) {
			idx := strings.LastIndex(name, sep)
			base := name[:idx]
			rest := name[idx+len(sep):]

			// Extract digits until next separator or extension
			partNum := ""
			for _, char := range rest {
				if char >= '0' && char <= '9' {
					partNum += string(char)
				} else {
					break
				}
			}

			if len(partNum) > 0 {
				return true, base, partNum
			}
		}
	}

	// Pattern: .001, .002 or .1, .2, .3 (at the very end)
	ext := filepath.Ext(name)
	if len(ext) >= 2 && ext[0] == '.' {
		// Verify if it's mostly digits
		partNum := ext[1:]
		isDigits := true
		if len(partNum) == 0 {
			isDigits = false
		}
		for _, char := range partNum {
			if char < '0' || char > '9' {
				isDigits = false
				break
			}
		}
		if isDigits {
			base := name[:len(name)-len(ext)]
			// Special case: if base still has an extension like .zip, remove it too for better set matching
			if subExt := filepath.Ext(base); subExt != "" {
				// but only if it's a known archive type
				if subExt == ".zip" || subExt == ".rar" || subExt == ".7z" || subExt == ".tar" || subExt == ".gz" {
					base = base[:len(base)-len(subExt)]
				}
			}
			return true, base, partNum
		}
	}

	return false, "", ""
}

// isSystemFolder checks if a path should be excluded from scanning
// Returns true for system folders like Recycle Bin, Trash, etc.
func isSystemFolder(path string) bool {
	lowerPath := strings.ToLower(path)

	// Windows system folders
	windowsFolders := []string{
		"$recycle.bin",
		"system volume information",
		"$windows.~bt",
		"$windows.~ws",
		"windows.old",
		"programdata",
		"recovery",
		"boot",
		"efi",
		"_trash",
	}

	// macOS system folders
	macFolders := []string{
		".trash",
		".trashes",
		".spotlight-v100",
		".fseventsd",
		".documentrevisions-v100",
		".temporaryitems",
		"library/caches",
		"library/logs",
	}

	// Linux system folders
	linuxFolders := []string{
		".trash-", // Matches .trash-1000, etc.
		".local/share/trash",
		"/proc",
		"/sys",
		"/dev",
		"/run",
		"/tmp",
		"/var/tmp",
	}

	allFolders := append(windowsFolders, macFolders...)
	allFolders = append(allFolders, linuxFolders...)

	for _, folder := range allFolders {
		if strings.Contains(lowerPath, folder) {
			return true
		}
	}

	return false
}

// ScanDirectory scans a directory for archive files
// ScanDirectory scans a directory for archive files
func ScanDirectory(dir string, recursive bool) ([]ArchiveFile, error) {
	var files []ArchiveFile

	// Progress counters (updated concurrently, read by the ticker below).
	var dirsDiscovered, dirsProcessed, filesFound int64

	// Background heartbeat so long full-system scans aren't silent.
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				log.Printf("   ⏳ [%s] dirs found: %d | scanned: %d | matches: %d",
					dir,
					atomic.LoadInt64(&dirsDiscovered),
					atomic.LoadInt64(&dirsProcessed),
					atomic.LoadInt64(&filesFound))
			}
		}
	}()
	defer close(progressDone)

	// First pass: collect all directory paths to scan
	var dirsToScan []string
	dirsToScan = append(dirsToScan, dir)

	for i := 0; i < len(dirsToScan); i++ {
		dirPath := dirsToScan[i]
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() && !isSystemFolder(filepath.Join(dirPath, entry.Name())) && recursive {
				dirsToScan = append(dirsToScan, filepath.Join(dirPath, entry.Name()))
			}
		}
		atomic.StoreInt64(&dirsDiscovered, int64(len(dirsToScan)))
	}

	// Second pass: parallelize file scanning across all directories
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a channel for directories to scan with buffering
	scanChan := make(chan string, len(dirsToScan))

	// Number of parallel workers - higher for I/O bound operations
	numWorkers := 24

	// Worker processes files in assigned directories
	worker := func() {
		defer wg.Done()
		for dirPath := range scanChan {
			atomic.AddInt64(&dirsProcessed, 1)
			entries, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}

			// Local buffer to reduce lock contention
			localFiles := []ArchiveFile{}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				fullPath := filepath.Join(dirPath, entry.Name())
				ext := strings.ToLower(filepath.Ext(fullPath))
				var fileType string

				switch {
				case ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz" || ext == ".bz2" || ext == ".xz" || ext == ".iso" || ext == ".cab":
					fileType = "archive"
				case ext == ".stl" || ext == ".obj" || ext == ".3ds" || ext == ".fbx" || ext == ".blend" || ext == ".step" || ext == ".stp" || ext == ".iges" || ext == ".igs" || ext == ".ply" || ext == ".off" || ext == ".3mf" || ext == ".glb" || ext == ".gltf":
					fileType = "model"
				}

				if fileType != "" {
					info, err := entry.Info()
					if err != nil {
						continue
					}

					localFiles = append(localFiles, ArchiveFile{
						Name:    entry.Name(),
						Path:    fullPath,
						Size:    info.Size(),
						Type:    fileType,
						ModTime: info.ModTime(),
					})
				}
			}

			// Append local results to global slice once
			if len(localFiles) > 0 {
				atomic.AddInt64(&filesFound, int64(len(localFiles)))
				mu.Lock()
				files = append(files, localFiles...)
				mu.Unlock()
			}
		}
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	// Queue all directories for processing
	for _, dir := range dirsToScan {
		scanChan <- dir
	}
	close(scanChan)

	// Wait for all workers to complete
	wg.Wait()

	return files, nil
}

// getArchiveType returns the archive type based on file extension
func getArchiveType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".iso", ".cab":
		return "archive"
	case ".stl", ".obj", ".3ds", ".fbx", ".blend", ".step", ".stp", ".iges", ".igs", ".ply", ".off", ".3mf", ".glb", ".gltf":
		return "model"
	case ".mp4", ".webm", ".mkv", ".avi", ".mov", ".wmv", ".flv":
		return "video"
	default:
		return ""
	}
}

// GroupBySize groups files by their size, filtering out multi-volume archive parts.
// Multi-volume parts are excluded from size-based grouping because they naturally
// share identical sizes (e.g., exactly 1GB or 1.5GB chunks), creating noise.
func GroupBySize(files []ArchiveFile) map[int64][]ArchiveFile {
	rawGroups := make(map[int64][]ArchiveFile)
	for _, file := range files {
		// Skip multi-volume parts in size-based analysis
		isPart, _, _ := file.IsMultiVolumePart()
		if isPart {
			continue
		}
		rawGroups[file.Size] = append(rawGroups[file.Size], file)
	}

	finalGroups := make(map[int64][]ArchiveFile)
	for size, group := range rawGroups {
		if len(group) >= 2 {
			finalGroups[size] = group
		}
	}

	return finalGroups
}

// PrintFileStats prints statistics about scanned files
func PrintFileStats(files []ArchiveFile) {
	stats := make(map[string]int)
	var totalSize int64

	for _, file := range files {
		stats[file.Type]++
		totalSize += file.Size
	}

	fmt.Printf("  • Archives: %d files\n", stats["archive"])
	fmt.Printf("  • 3D Models: %d files\n", stats["model"])
	fmt.Printf("  • Videos: %d files\n", stats["video"])
	fmt.Printf("  • Total size: %s\n", formatBytes(totalSize))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetRootPaths returns a list of root paths to scan for full-system scans.
func GetRootPaths() []string {
	if runtime.GOOS == "windows" {
		var roots []string
		for c := 'C'; c <= 'Z'; c++ {
			p := fmt.Sprintf("%c:\\", c)
			if _, err := os.Stat(p); err == nil {
				roots = append(roots, p)
			}
		}
		if len(roots) == 0 {
			roots = append(roots, "C:\\")
		}
		return roots
	}
	return []string{"/"}
}

// ScanMultiplePaths scans multiple root paths and aggregates results.
// It first tries the OS-specific indexed database (Everything on Windows,
// Spotlight on macOS, locate on Linux) which returns in seconds. If that is
// unavailable or fails, it falls back to a manual directory walk (with progress).
func ScanMultiplePaths(paths []string, recursive bool) ([]ArchiveFile, error) {
	switch runtime.GOOS {
	case "windows":
		if IsEverythingAvailable() {
			log.Println("⚡ Everything database detected - using fast indexed search...")
			start := time.Now()
			files, err := ScanWithEverything()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using Everything in %s", len(files), time.Since(start).Round(time.Millisecond))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Everything search failed, falling back to directory scan: %v", err)
			}
		} else {
			log.Println("ℹ️  Everything CLI (es.exe) not found — using manual walk. Install 'es.exe' from voidtools into C:\\Program Files\\Everything\\ or your PATH for fast indexed scans.")
		}

	case "darwin":
		if IsMdfindAvailable() {
			log.Println("⚡ Spotlight database detected - using fast indexed search...")
			start := time.Now()
			files, err := ScanWithMdfind()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using Spotlight in %s", len(files), time.Since(start).Round(time.Millisecond))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Spotlight search failed, falling back to directory scan: %v", err)
			}
		}

	case "linux":
		if IsLocateAvailable() {
			log.Println("⚡ Locate database detected - using fast indexed search...")
			start := time.Now()
			files, err := ScanWithLocate()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using locate in %s", len(files), time.Since(start).Round(time.Millisecond))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Locate search failed, falling back to directory scan: %v", err)
			}
		}
	}

	// Fallback: manual directory walk, one root at a time (with per-root progress).
	var all []ArchiveFile
	for idx, p := range paths {
		log.Printf("🔍 [%d/%d] Scanning root: %s", idx+1, len(paths), p)
		start := time.Now()
		files, err := ScanDirectory(p, recursive)
		if err != nil {
			log.Printf("⚠️  [%d/%d] Failed to scan %s: %v", idx+1, len(paths), p, err)
			continue
		}
		all = append(all, files...)
		log.Printf("✅ [%d/%d] %s done: %d matches (%d total) in %s",
			idx+1, len(paths), p, len(files), len(all), time.Since(start).Round(time.Second))
	}
	return all, nil
}

// IsEverythingAvailable checks if Everything's CLI (es.exe) is available on Windows.
func IsEverythingAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	possiblePaths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Everything", "es.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Everything", "es.exe"),
		"C:\\Program Files\\Everything\\es.exe",
		"C:\\Program Files (x86)\\Everything\\es.exe",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	_, err := exec.LookPath("es.exe")
	return err == nil
}

// GetEverythingPath returns the path to es.exe if available, else the bare name.
func GetEverythingPath() string {
	possiblePaths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Everything", "es.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Everything", "es.exe"),
		"C:\\Program Files\\Everything\\es.exe",
		"C:\\Program Files (x86)\\Everything\\es.exe",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "es.exe" // fall back to PATH lookup
}

// ScanWithEverything scans for archive/model/video files using the Everything index.
func ScanWithEverything() ([]ArchiveFile, error) {
	if !IsEverythingAvailable() {
		return nil, fmt.Errorf("Everything is not available")
	}

	esPath := GetEverythingPath()
	var files []ArchiveFile

	searchExts := []string{"*.stl", "*.obj", "*.zip", "*.rar", "*.7z"}

	var filesFound int64
	mu := sync.Mutex{}
	var wg sync.WaitGroup

	// Progress heartbeat (Everything is fast, but keep it consistent with the walk).
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				log.Printf("   ⏳ [Everything] matches so far: %d", atomic.LoadInt64(&filesFound))
			}
		}
	}()
	defer close(progressDone)

	searchExtension := func(ext string) {
		defer wg.Done()

		// ES search syntax: the "file:" function restricts results to files only.
		// (The "-no-folders" switch is not supported by older es.exe versions and
		// makes es.exe print its help text instead of results.)
		cmd := exec.Command(esPath, "file:"+ext)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("⚠️ Error querying Everything for %s: %v", ext, err)
			return
		}
		if err := cmd.Start(); err != nil {
			log.Printf("⚠️ Error starting Everything query for %s: %v", ext, err)
			return
		}

		scanner := bufio.NewScanner(stdout)
		// Allow very long path lines.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" || isSystemFolder(filePath) {
				continue
			}
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				continue
			}
			fileType := getArchiveType(filePath)
			if fileType != "" {
				atomic.AddInt64(&filesFound, 1)
				mu.Lock()
				files = append(files, ArchiveFile{
					Name:    filepath.Base(filePath),
					Path:    filePath,
					Size:    info.Size(),
					Type:    fileType,
					ModTime: info.ModTime(),
				})
				mu.Unlock()
			}
		}
		cmd.Wait()
	}

	for _, ext := range searchExts {
		wg.Add(1)
		go searchExtension(ext)
	}
	wg.Wait()

	return files, nil
}

// IsLocateAvailable checks if the locate command is available (Linux/macOS).
func IsLocateAvailable() bool {
	_, err := exec.LookPath("locate")
	return err == nil
}

// ScanWithLocate scans for archive/model/video files using the locate command.
func ScanWithLocate() ([]ArchiveFile, error) {
	if !IsLocateAvailable() {
		return nil, fmt.Errorf("locate command is not available")
	}

	var files []ArchiveFile
	mu := sync.Mutex{}

	archiveExts := []string{"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "iso", "cab"}
	modelExts := []string{
		"stl", "obj", "3ds", "fbx", "blend", "step", "stp", "iges", "igs",
		"ply", "off", "3mf", "glb", "gltf",
	}
	videoExts := []string{"mp4", "webm", "mkv", "avi", "mov", "wmv", "flv"}
	allExts := append(append(archiveExts, modelExts...), videoExts...)

	searchExtension := func(ext string) {
		cmd := exec.Command("locate", "-i", "*."+ext)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" || isSystemFolder(filePath) {
				continue
			}
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				continue
			}
			fileType := getArchiveType(filePath)
			if fileType != "" {
				mu.Lock()
				files = append(files, ArchiveFile{
					Name:    filepath.Base(filePath),
					Path:    filePath,
					Size:    info.Size(),
					Type:    fileType,
					ModTime: info.ModTime(),
				})
				mu.Unlock()
			}
		}
		cmd.Wait()
	}

	for _, ext := range allExts {
		searchExtension(ext)
	}

	return files, nil
}

// IsMdfindAvailable checks if mdfind (Spotlight) is available on macOS.
func IsMdfindAvailable() bool {
	_, err := exec.LookPath("mdfind")
	return err == nil
}

// ScanWithMdfind scans for archive/model/video files using Spotlight (macOS).
func ScanWithMdfind() ([]ArchiveFile, error) {
	if !IsMdfindAvailable() {
		return nil, fmt.Errorf("mdfind command is not available")
	}

	var files []ArchiveFile
	mu := sync.Mutex{}

	archiveExts := []string{"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "iso", "cab"}
	modelExts := []string{
		"stl", "obj", "3ds", "fbx", "blend", "step", "stp", "iges", "igs",
		"ply", "off", "3mf", "glb", "gltf",
	}
	videoExts := []string{"mp4", "webm", "mkv", "avi", "mov", "wmv", "flv"}
	allExts := append(append(archiveExts, modelExts...), videoExts...)

	searchExtension := func(ext string) {
		query := fmt.Sprintf("filename:%s.%s", "*", ext)
		cmd := exec.Command("mdfind", query)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" || isSystemFolder(filePath) {
				continue
			}
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				continue
			}
			fileType := getArchiveType(filePath)
			if fileType != "" {
				mu.Lock()
				files = append(files, ArchiveFile{
					Name:    filepath.Base(filePath),
					Path:    filePath,
					Size:    info.Size(),
					Type:    fileType,
					ModTime: info.ModTime(),
				})
				mu.Unlock()
			}
		}
		cmd.Wait()
	}

	for _, ext := range allExts {
		searchExtension(ext)
	}

	return files, nil
}
