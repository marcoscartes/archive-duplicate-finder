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

// PreviewInfo represents information about a previewable file inside an archive
type PreviewInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

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

// ListPreviewsInArchive returns a list of all files that can be used as previews
func ListPreviewsInArchive(archivePath string) ([]PreviewInfo, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	var files []PreviewInfo
	var err error

	switch ext {
	case ".zip":
		files, err = listFilesZIP(archivePath)
	case ".rar":
		files, err = listFilesRAR(archivePath)
	case ".7z":
		files, err = listFiles7Z(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}

	if err != nil {
		return nil, err
	}

	var previews []PreviewInfo
	for _, f := range files {
		if isImageFile(f.Path) || isSTLFile(f.Path) || isVideoFile(f.Path) {
			previews = append(previews, f)
		}
	}
	return previews, nil
}

// FindFirstImageInArchive returns the contents of the first image (jpg, jpeg, png) found in the archive
// Deprecated: Use FindLargestImageInArchive for better quality previews
func FindFirstImageInArchive(archivePath string) ([]byte, string, error) {
	return FindLargestImageInArchive(archivePath)
}

// FindLargestImageInArchive returns the contents of the largest image file in the archive
// This is useful for finding high-quality render previews
func FindLargestImageInArchive(archivePath string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".zip":
		return findLargestImageZIP(archivePath)
	case ".rar":
		return findLargestImageRAR(archivePath)
	case ".7z":
		return findLargestImage7Z(archivePath)
	default:
		return nil, "", fmt.Errorf("unsupported archive format: %s", ext)
	}
}

// FindPreviewInArchive returns preview content from archive:
// 1. First tries to find the largest JPG/PNG image
// 2. If no images, looks for the largest MP4/WebM video
// 3. If no videos, looks for STL files containing keywords: "full", "whole", "body", "complete"
// 4. Fallback: finds the largest STL file if no keywords match
func FindPreviewInArchive(archivePath string) ([]byte, string, error) {
	// Try to find largest image first
	data, filename, err := FindLargestImageInArchive(archivePath)
	if err == nil {
		return data, filename, nil
	}

	// No image, try to find largest video
	data, filename, err = FindLargestVideoInArchive(archivePath)
	if err == nil {
		return data, filename, nil
	}

	// No video found, try to find STL with keywords
	ext := strings.ToLower(filepath.Ext(archivePath))

	var stlData []byte
	var stlName string

	switch ext {
	case ".zip":
		stlData, stlName, err = findKeywordSTLZIP(archivePath)
		if err != nil {
			// Fallback to largest STL
			stlData, stlName, err = findLargestSTLZIP(archivePath)
		}
	case ".rar":
		stlData, stlName, err = findKeywordSTLRAR(archivePath)
		if err != nil {
			stlData, stlName, err = findLargestSTLRAR(archivePath)
		}
	case ".7z":
		stlData, stlName, err = findKeywordSTL7Z(archivePath)
		if err != nil {
			stlData, stlName, err = findLargestSTL7Z(archivePath)
		}
	default:
		return nil, "", fmt.Errorf("no preview found in archive")
	}

	if err == nil && len(stlData) > 0 {
		return stlData, stlName, nil
	}

	return nil, "", fmt.Errorf("no preview found in archive")
}

// FindPreviewPathInArchive returns the internal path of the best preview candidate
func FindPreviewPathInArchive(archivePath string) (string, error) {
	previews, err := ListPreviewsInArchive(archivePath)
	if err != nil {
		return "", err
	}
	if len(previews) == 0 {
		return "", fmt.Errorf("no preview found")
	}

	// 1. Find largest image
	var bestImage string
	var maxImgSize int64
	for _, f := range previews {
		if isImageFile(f.Path) && f.Size > maxImgSize {
			bestImage = f.Path
			maxImgSize = f.Size
		}
	}
	if bestImage != "" {
		return bestImage, nil
	}

	// 2. Find largest video
	var bestVideo string
	var maxVidSize int64
	for _, f := range previews {
		if isVideoFile(f.Path) && f.Size > maxVidSize {
			bestVideo = f.Path
			maxVidSize = f.Size
		}
	}
	if bestVideo != "" {
		return bestVideo, nil
	}

	// 3. Find STL with keywords
	for _, f := range previews {
		if isSTLFile(f.Path) && hasKeyword(f.Path) {
			return f.Path, nil
		}
	}

	// 4. Find largest STL
	var bestSTL string
	var maxSTLSize int64
	for _, f := range previews {
		if isSTLFile(f.Path) && f.Size > maxSTLSize {
			bestSTL = f.Path
			maxSTLSize = f.Size
		}
	}
	if bestSTL != "" {
		return bestSTL, nil
	}

	return "", fmt.Errorf("no preview found")
}

func isSTLFile(filename string) bool {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "__macosx") {
		return false
	}
	return strings.HasSuffix(lower, ".stl")
}

func hasKeyword(filename string) bool {
	lower := strings.ToLower(filename)
	keywords := []string{"full", "whole", "body", "complete", "merged", "single"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func findKeywordSTLZIP(archivePath string) ([]byte, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if !file.FileInfo().IsDir() && isSTLFile(name) && hasKeyword(name) {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil && len(data) > 0 {
				return data, file.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no STL with keywords found")
}

func findLargestSTLZIP(archivePath string) ([]byte, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize uint64

	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if !file.FileInfo().IsDir() && isSTLFile(name) {
			if file.UncompressedSize64 > largestSize {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = uint64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no STL found")
	}
	return largestData, largestName, nil
}

func findKeywordSTLRAR(archivePath string) ([]byte, string, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		name := strings.ReplaceAll(header.Name, "\\", "/")
		if !header.IsDir && isSTLFile(name) && hasKeyword(name) {
			data, err := io.ReadAll(reader)
			if err == nil && len(data) > 0 {
				return data, header.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no STL with keywords found")
}

func findLargestSTLRAR(archivePath string) ([]byte, string, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		name := strings.ReplaceAll(header.Name, "\\", "/")
		if !header.IsDir && isSTLFile(name) {
			if header.UnPackedSize > largestSize {
				data, err := io.ReadAll(reader)
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = header.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no STL found")
	}
	return largestData, largestName, nil
}

func findKeywordSTL7Z(archivePath string) ([]byte, string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if !file.FileInfo().IsDir() && isSTLFile(name) && hasKeyword(name) {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil && len(data) > 0 {
				return data, file.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no STL with keywords found")
}

func findLargestSTL7Z(archivePath string) ([]byte, string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize uint64

	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if !file.FileInfo().IsDir() && isSTLFile(name) {
			if file.UncompressedSize > largestSize {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = uint64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no STL found")
	}
	return largestData, largestName, nil
}

func isImageFile(filename string) bool {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "__macosx") || strings.Contains(lower, "@eadir") {
		return false
	}
	ext := filepath.Ext(lower)
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp"
}

func isVideoFile(filename string) bool {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "__macosx") || strings.Contains(lower, "@eadir") {
		return false
	}
	ext := filepath.Ext(lower)
	return ext == ".mp4" || ext == ".webm" || ext == ".mkv" || ext == ".mov" || ext == ".avi"
}

// FindLargestVideoInArchive returns the contents of the largest video file in the archive
func FindLargestVideoInArchive(archivePath string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".zip":
		return findLargestFileWithFilter(archivePath, isVideoFile)
	case ".rar":
		return findLargestFileWithFilterRAR(archivePath, isVideoFile)
	case ".7z":
		return findLargestFileWithFilter7Z(archivePath, isVideoFile)
	default:
		return nil, "", fmt.Errorf("unsupported archive format: %s", ext)
	}
}

func findLargestFileWithFilter(archivePath string, filter func(string) bool) ([]byte, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && filter(file.Name) {
			if file.UncompressedSize64 > uint64(largestSize) {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no matching file found")
	}
	return largestData, largestName, nil
}

func findLargestFileWithFilterRAR(archivePath string, filter func(string) bool) ([]byte, string, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if !header.IsDir && filter(header.Name) {
			if header.UnPackedSize > largestSize {
				data, err := io.ReadAll(reader)
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = header.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no matching file found")
	}
	return largestData, largestName, nil
}

func findLargestFileWithFilter7Z(archivePath string, filter func(string) bool) ([]byte, string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && filter(file.Name) {
			if int64(file.UncompressedSize) > largestSize {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no matching file found")
	}
	return largestData, largestName, nil
}

func findLargestImageZIP(archivePath string) ([]byte, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isImageFile(file.Name) {
			// Check if this image is larger than the current largest
			if file.UncompressedSize64 > uint64(largestSize) {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no image found")
	}
	return largestData, largestName, nil
}

// Keep old function for backwards compatibility
func findFirstImageZIP(archivePath string) ([]byte, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isImageFile(file.Name) {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil {
				return data, file.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no image found")
}

func findLargestImageRAR(archivePath string) ([]byte, string, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if !header.IsDir && isImageFile(header.Name) {
			if header.UnPackedSize > largestSize {
				data, err := io.ReadAll(reader)
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = header.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no image found")
	}
	return largestData, largestName, nil
}

// Keep old function for backwards compatibility
func findFirstImageRAR(archivePath string) ([]byte, string, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if !header.IsDir && isImageFile(header.Name) {
			data, err := io.ReadAll(reader)
			if err == nil {
				return data, header.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no image found")
}

func findLargestImage7Z(archivePath string) ([]byte, string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var largestData []byte
	var largestName string
	var largestSize int64

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isImageFile(file.Name) {
			if int64(file.UncompressedSize) > largestSize {
				rc, err := file.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(data) > 0 {
					largestData = data
					largestName = file.Name
					largestSize = int64(len(data))
				}
			}
		}
	}

	if largestData == nil {
		return nil, "", fmt.Errorf("no image found")
	}
	return largestData, largestName, nil
}

// Keep old function for backwards compatibility
func findFirstImage7Z(archivePath string) ([]byte, string, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isImageFile(file.Name) {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err == nil {
				return data, file.Name, nil
			}
		}
	}
	return nil, "", fmt.Errorf("no image found")
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

func listFilesZIP(archivePath string) ([]PreviewInfo, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var files []PreviewInfo
	for _, f := range reader.File {
		if !f.FileInfo().IsDir() {
			files = append(files, PreviewInfo{
				Path: f.Name,
				Size: int64(f.UncompressedSize64),
			})
		}
	}
	return files, nil
}

func listFilesRAR(archivePath string) ([]PreviewInfo, error) {
	reader, err := rardecode.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var files []PreviewInfo
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !header.IsDir {
			files = append(files, PreviewInfo{
				Path: header.Name,
				Size: header.UnPackedSize,
			})
		}
	}
	return files, nil
}

func listFiles7Z(archivePath string) ([]PreviewInfo, error) {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var files []PreviewInfo
	for _, f := range reader.File {
		if !f.FileInfo().IsDir() {
			files = append(files, PreviewInfo{
				Path: f.Name,
				Size: int64(f.UncompressedSize),
			})
		}
	}
	return files, nil
}
