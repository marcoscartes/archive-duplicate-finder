package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ScanDirectory scans a directory for archive files
func ScanDirectory(dir string, recursive bool) ([]ArchiveFile, error) {
	var files []ArchiveFile

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// If not recursive and not the root directory, skip
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is an archive
		archiveType := getArchiveType(path)
		if archiveType != "" {
			files = append(files, ArchiveFile{
				Name:    info.Name(),
				Path:    path,
				Size:    info.Size(),
				Type:    archiveType,
				ModTime: info.ModTime(),
			})
		}

		return nil
	})

	return files, err
}

// getArchiveType returns the archive type based on file extension
func getArchiveType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".zip":
		return "zip"
	case ".rar":
		return "rar"
	case ".7z":
		return "7z"
	default:
		return ""
	}
}

// GroupBySize groups files by their size
func GroupBySize(files []ArchiveFile) map[int64][]ArchiveFile {
	groups := make(map[int64][]ArchiveFile)

	for _, file := range files {
		groups[file.Size] = append(groups[file.Size], file)
	}

	return groups
}

// PrintFileStats prints statistics about scanned files
func PrintFileStats(files []ArchiveFile) {
	stats := make(map[string]int)
	var totalSize int64

	for _, file := range files {
		stats[file.Type]++
		totalSize += file.Size
	}

	fmt.Printf("  • ZIP: %d files\n", stats["zip"])
	fmt.Printf("  • RAR: %d files\n", stats["rar"])
	fmt.Printf("  • 7Z: %d files\n", stats["7z"])
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
