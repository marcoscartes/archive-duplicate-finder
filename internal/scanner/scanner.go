package scanner

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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

// GetRootPaths returns the root directories to scan based on the operating system
func GetRootPaths() []string {
	var roots []string
	osType := os.Getenv("GOOS")
	if osType == "" {
		// Use runtime.GOOS if environment variable is not set
		osType = os.Getenv("GOOS")
		if osType == "" {
			// Fallback: detect OS
			switch runtime.GOOS {
			case "windows":
				osType = "windows"
			case "darwin":
				osType = "darwin"
			default:
				osType = "linux"
			}
		}
	}

	switch runtime.GOOS {
	case "windows":
		// Get all drive letters on Windows
		for i := 'A'; i <= 'Z'; i++ {
			drive := string(i) + ":"
			_, err := os.Stat(drive)
			if err == nil {
				roots = append(roots, drive)
			}
		}
	case "darwin":
		// On macOS, scan /Users and /Volumes for mounted volumes
		roots = append(roots, "/Users")
		roots = append(roots, "/Volumes")
	case "linux":
		// On Linux, start from home directory and /mnt for mounted volumes
		home, err := os.UserHomeDir()
		if err == nil {
			roots = append(roots, home)
		}
		roots = append(roots, "/mnt")
		roots = append(roots, "/media")
	default:
		// Fallback to current directory
		roots = append(roots, ".")
	}

	return roots
}

// ScanMultiplePaths scans multiple root paths for archive and model files
func ScanMultiplePaths(paths []string, recursive bool) ([]ArchiveFile, error) {
	// Try to use OS-specific indexed databases first
	switch runtime.GOOS {
	case "windows":
		if IsEverythingAvailable() {
			log.Println("⚡ Everything database detected - using fast indexed search...")
			files, err := ScanWithEverything()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using Everything", len(files))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Everything search failed, falling back to directory scan: %v", err)
			}
		}

	case "darwin":
		if IsMdfindAvailable() {
			log.Println("⚡ Spotlight database detected - using fast indexed search...")
			files, err := ScanWithMdfind()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using Spotlight", len(files))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Spotlight search failed, falling back to directory scan: %v", err)
			}
		}

	case "linux":
		if IsLocateAvailable() {
			log.Println("⚡ Locate database detected - using fast indexed search...")
			files, err := ScanWithLocate()
			if err == nil && len(files) > 0 {
				log.Printf("✅ Found %d files using locate", len(files))
				return files, nil
			}
			if err != nil {
				log.Printf("⚠️ Locate search failed, falling back to directory scan: %v", err)
			}
		}
	}

	// Fallback to normal directory scanning
	var files []ArchiveFile
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a channel to limit concurrent path scans
	pathChan := make(chan string, len(paths))
	numWorkers := 4 // Use fewer workers for multiple paths to avoid overwhelming the system

	worker := func() {
		defer wg.Done()
		for path := range pathChan {
			scanned, err := ScanDirectory(path, recursive)
			if err != nil {
				continue
			}
			if len(scanned) > 0 {
				mu.Lock()
				files = append(files, scanned...)
				mu.Unlock()
			}
		}
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	// Queue all paths for processing
	for _, path := range paths {
		pathChan <- path
	}
	close(pathChan)

	// Wait for all workers to complete
	wg.Wait()

	return files, nil
}

// IsEverythingAvailable checks if Everything is installed and running on Windows
func IsEverythingAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Check if es.exe exists in Program Files
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

	// Try to find es.exe in PATH
	_, err := exec.LookPath("es.exe")
	return err == nil
}

// GetEverythingPath returns the path to es.exe if available
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

	return "es.exe" // Return the default name to be found in PATH
}

// ScanWithEverything scans for archive/model files using Everything database
func ScanWithEverything() ([]ArchiveFile, error) {
	if !IsEverythingAvailable() {
		return nil, fmt.Errorf("Everything is not available")
	}

	esPath := GetEverythingPath()
	var files []ArchiveFile

	// Archive extensions
	archiveExts := []string{
		"*.zip", "*.rar", "*.7z", "*.tar", "*.gz", "*.bz2", "*.xz", "*.iso", "*.cab",
	}

	// Model extensions
	modelExts := []string{
		"*.stl", "*.obj", "*.3ds", "*.fbx", "*.blend", "*.step", "*.stp", "*.iges", "*.igs",
		"*.ply", "*.off", "*.3mf", "*.glb", "*.gltf",
	}

	// Video extensions
	videoExts := []string{
		"*.mp4", "*.webm", "*.mkv", "*.avi", "*.mov", "*.wmv", "*.flv",
	}

	allExts := append(append(archiveExts, modelExts...), videoExts...)

	// Query Everything for each extension
	mu := sync.Mutex{}
	var wg sync.WaitGroup

	searchExtension := func(ext string) {
		defer wg.Done()

		// Use Everything CLI: es -no-folders filename_pattern
		cmd := exec.Command(esPath, "-no-folders", ext)
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
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" {
				continue
			}

			// Skip system folders
			if isSystemFolder(filePath) {
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil {
				continue // File might have been deleted
			}

			if info.IsDir() {
				continue // Skip directories
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

	// Search for all extensions concurrently
	for _, ext := range allExts {
		wg.Add(1)
		go searchExtension(ext)
	}

	wg.Wait()

	return files, nil
}

// IsLocateAvailable checks if locate command is available on Linux/macOS
func IsLocateAvailable() bool {
	_, err := exec.LookPath("locate")
	return err == nil
}

// ScanWithLocate scans for archive/model files using the locate command (Linux/macOS)
func ScanWithLocate() ([]ArchiveFile, error) {
	if !IsLocateAvailable() {
		return nil, fmt.Errorf("locate command is not available")
	}

	var files []ArchiveFile
	mu := sync.Mutex{}

	// Archive extensions
	archiveExts := []string{
		"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "iso", "cab",
	}

	// Model extensions
	modelExts := []string{
		"stl", "obj", "3ds", "fbx", "blend", "step", "stp", "iges", "igs",
		"ply", "off", "3mf", "glb", "gltf",
	}

	// Video extensions
	videoExts := []string{
		"mp4", "webm", "mkv", "avi", "mov", "wmv", "flv",
	}

	allExts := append(append(archiveExts, modelExts...), videoExts...)

	searchExtension := func(ext string) {
		// Use locate to search for files with extension
		cmd := exec.Command("locate", "-i", "*."+ext)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}

		if err := cmd.Start(); err != nil {
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" {
				continue
			}

			// Skip system folders
			if isSystemFolder(filePath) {
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			if info.IsDir() {
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

	// Search extensions sequentially with locate (it's fast enough)
	for _, ext := range allExts {
		searchExtension(ext)
	}

	return files, nil
}

// IsMdfindAvailable checks if mdfind (Spotlight) is available on macOS
func IsMdfindAvailable() bool {
	_, err := exec.LookPath("mdfind")
	return err == nil
}

// ScanWithMdfind scans for archive/model files using Spotlight (macOS)
func ScanWithMdfind() ([]ArchiveFile, error) {
	if !IsMdfindAvailable() {
		return nil, fmt.Errorf("mdfind command is not available")
	}

	var files []ArchiveFile
	mu := sync.Mutex{}

	// Archive extensions
	archiveExts := []string{
		"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "iso", "cab",
	}

	// Model extensions
	modelExts := []string{
		"stl", "obj", "3ds", "fbx", "blend", "step", "stp", "iges", "igs",
		"ply", "off", "3mf", "glb", "gltf",
	}

	// Video extensions
	videoExts := []string{
		"mp4", "webm", "mkv", "avi", "mov", "wmv", "flv",
	}

	allExts := append(append(archiveExts, modelExts...), videoExts...)

	searchExtension := func(ext string) {
		// mdfind searches Spotlight index: -name looks for files, -onlyin limits scope
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
		for scanner.Scan() {
			filePath := strings.TrimSpace(scanner.Text())
			if filePath == "" {
				continue
			}

			// Skip system folders
			if isSystemFolder(filePath) {
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			if info.IsDir() {
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

	// Search extensions sequentially with mdfind
	for _, ext := range allExts {
		searchExtension(ext)
	}

	return files, nil
}
