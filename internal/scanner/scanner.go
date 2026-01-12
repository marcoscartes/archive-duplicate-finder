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
	case ".stl":
		return "stl"
	case ".obj":
		return "obj"
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
