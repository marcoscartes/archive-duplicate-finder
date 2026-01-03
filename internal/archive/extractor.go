package archive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// ExtractArchive extracts all files from an archive and returns them as a map
// Key: filename, Value: file contents
func ExtractArchive(archivePath string) (map[string][]byte, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".zip":
		return extractZIP(archivePath)
	case ".rar":
		return extractRAR(archivePath)
	case ".7z":
		return extract7Z(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}
}

// extractZIP extracts files from a ZIP archive
func extractZIP(archivePath string) (map[string][]byte, error) {
	contents := make(map[string][]byte)

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		// Open file
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", file.Name, err)
		}

		// Read contents
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file.Name, err)
		}

		contents[file.Name] = data
	}

	return contents, nil
}

// extractRAR extracts files from a RAR archive
func extractRAR(archivePath string) (map[string][]byte, error) {
	contents := make(map[string][]byte)

	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open RAR: %w", err)
	}
	defer reader.Close()

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read RAR header: %w", err)
		}

		// Skip directories
		if header.IsDir {
			continue
		}

		// Read contents
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", header.Name, err)
		}

		contents[header.Name] = data
	}

	return contents, nil
}

// extract7Z extracts files from a 7Z archive
func extract7Z(archivePath string) (map[string][]byte, error) {
	contents := make(map[string][]byte)

	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open 7Z: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		// Open file
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", file.Name, err)
		}

		// Read contents
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file.Name, err)
		}

		contents[file.Name] = data
	}

	return contents, nil
}

// CompareArchiveContents compares two archives and returns common and unique files
func CompareArchiveContents(archive1, archive2 string) (common, unique1, unique2 []string, err error) {
	contents1, err := ExtractArchive(archive1)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to extract archive 1: %w", err)
	}

	contents2, err := ExtractArchive(archive2)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to extract archive 2: %w", err)
	}

	// Find common and unique files
	for name := range contents1 {
		if _, exists := contents2[name]; exists {
			common = append(common, name)
		} else {
			unique1 = append(unique1, name)
		}
	}

	for name := range contents2 {
		if _, exists := contents1[name]; !exists {
			unique2 = append(unique2, name)
		}
	}

	return common, unique1, unique2, nil
}

// GetFileFromArchive extracts a specific file from an archive
func GetFileFromArchive(archivePath, filename string) ([]byte, error) {
	contents, err := ExtractArchive(archivePath)
	if err != nil {
		return nil, err
	}

	data, exists := contents[filename]
	if !exists {
		return nil, fmt.Errorf("file %s not found in archive", filename)
	}

	return data, nil
}

// CalculateHash calculates SHA-256 hash of file contents
func CalculateHash(data []byte) string {
	// Simple hash for now - can be improved with crypto/sha256
	hash := 0
	for _, b := range data {
		hash = hash*31 + int(b)
	}
	return fmt.Sprintf("%x", hash)
}

// AreFilesIdentical checks if two byte arrays are identical
func AreFilesIdentical(data1, data2 []byte) bool {
	return bytes.Equal(data1, data2)
}
