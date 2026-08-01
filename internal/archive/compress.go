package archive

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// RecompressFile creates a ZIP archive containing the input file and preserves the original filename.
// It is intended for STL, OBJ, and JPG assets when a smaller, more portable archive is desired.
func RecompressFile(inputPath string, outputPath string) (string, error) {
	if inputPath == "" {
		return "", fmt.Errorf("input path cannot be empty")
	}
	if outputPath == "" {
		return "", fmt.Errorf("output path cannot be empty")
	}

	inputInfo, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat input file: %w", err)
	}
	if inputInfo.IsDir() {
		return "", fmt.Errorf("input path must be a file")
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	archiveFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output archive: %w", err)
	}
	defer archiveFile.Close()

	zipWriter := zip.NewWriter(archiveFile)
	defer zipWriter.Close()

	archiveName := filepath.Base(inputPath)
	archiveEntry, err := zipWriter.Create(archiveName)
	if err != nil {
		return "", fmt.Errorf("failed to create zip entry: %w", err)
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	if _, err := io.Copy(archiveEntry, inputFile); err != nil {
		return "", fmt.Errorf("failed to write zip entry: %w", err)
	}

	return outputPath, nil
}

// CompressionResult stores the output path and size for a compression comparison.
type CompressionResult struct {
	Format string
	Path   string
	Size   int64
	Err    error
}

// RecompressArchive repacks an existing archive into a new ZIP archive.
// It is intended for testing whether a second-pass compression improves size.
func RecompressArchive(inputPath string, outputPath string) (string, error) {
	if inputPath == "" {
		return "", fmt.Errorf("input path cannot be empty")
	}
	if outputPath == "" {
		return "", fmt.Errorf("output path cannot be empty")
	}

	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext != ".zip" && ext != ".rar" && ext != ".7z" {
		return "", fmt.Errorf("unsupported archive format for recompression: %s", ext)
	}

	files, err := ExtractArchive(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read source archive: %w", err)
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	return outputPath, writeZIPArchive(outputPath, files)
}

// CompareArchiveRecompression evaluates multiple output formats and compression levels for the same archive contents.
// ZIP is always attempted. 7Z and RAR are added when their CLI tools are available.
func CompareArchiveRecompression(inputPath string, outputDir string) ([]CompressionResult, error) {
	if inputPath == "" {
		return nil, fmt.Errorf("input path cannot be empty")
	}

	files, err := ExtractArchive(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source archive: %w", err)
	}

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	results := []CompressionResult{}

	for _, level := range []struct {
		name string
		mode int
	}{
		{name: "zip-fast", mode: 1},
		{name: "zip-normal", mode: 6},
		{name: "zip-max", mode: 9},
	} {
		zipPath := filepath.Join(outputDir, baseName+"_"+level.name+".zip")
		if err := writeZIPArchiveWithLevel(zipPath, files, level.mode); err != nil {
			results = append(results, CompressionResult{Format: level.name, Path: zipPath, Err: err})
		} else {
			info, statErr := os.Stat(zipPath)
			if statErr != nil {
				results = append(results, CompressionResult{Format: level.name, Path: zipPath, Err: statErr})
			} else {
				results = append(results, CompressionResult{Format: level.name, Path: zipPath, Size: info.Size()})
			}
		}
	}

	if _, err := exec.LookPath("7z"); err == nil {
		for _, level := range []struct {
			name string
			mode string
		}{
			{name: "7z-fast", mode: "-mx=1"},
			{name: "7z-normal", mode: "-mx=5"},
			{name: "7z-max", mode: "-mx=9"},
		} {
			sevenZipPath := filepath.Join(outputDir, baseName+"_"+level.name+".7z")
			if err := writeSevenZipArchiveWithLevel(sevenZipPath, files, level.mode); err != nil {
				results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Err: err})
			} else {
				info, statErr := os.Stat(sevenZipPath)
				if statErr != nil {
					results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Err: statErr})
				} else {
					results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Size: info.Size()})
				}
			}
		}
	} else if _, err := exec.LookPath("7za"); err == nil {
		for _, level := range []struct {
			name string
			mode string
		}{
			{name: "7z-fast", mode: "-mx=1"},
			{name: "7z-normal", mode: "-mx=5"},
			{name: "7z-max", mode: "-mx=9"},
		} {
			sevenZipPath := filepath.Join(outputDir, baseName+"_"+level.name+".7z")
			if err := writeSevenZipArchiveWithLevel(sevenZipPath, files, level.mode); err != nil {
				results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Err: err})
			} else {
				info, statErr := os.Stat(sevenZipPath)
				if statErr != nil {
					results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Err: statErr})
				} else {
					results = append(results, CompressionResult{Format: level.name, Path: sevenZipPath, Size: info.Size()})
				}
			}
		}
	}

	if _, err := exec.LookPath("rar"); err == nil {
		for _, level := range []struct {
			name string
			mode string
		}{
			{name: "rar-fast", mode: "-m1"},
			{name: "rar-normal", mode: "-m3"},
			{name: "rar-max", mode: "-m5"},
		} {
			rarPath := filepath.Join(outputDir, baseName+"_"+level.name+".rar")
			if err := writeRarArchiveWithLevel(rarPath, files, level.mode); err != nil {
				results = append(results, CompressionResult{Format: level.name, Path: rarPath, Err: err})
			} else {
				info, statErr := os.Stat(rarPath)
				if statErr != nil {
					results = append(results, CompressionResult{Format: level.name, Path: rarPath, Err: statErr})
				} else {
					results = append(results, CompressionResult{Format: level.name, Path: rarPath, Size: info.Size()})
				}
			}
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Err != nil && results[j].Err == nil {
			return false
		}
		if results[i].Err == nil && results[j].Err != nil {
			return true
		}
		if results[i].Err != nil && results[j].Err != nil {
			return results[i].Format < results[j].Format
		}
		return results[i].Size < results[j].Size
	})

	return results, nil
}

func writeZIPArchive(outputPath string, files map[string][]byte) error {
	return writeZIPArchiveWithLevel(outputPath, files, 6)
}

func writeZIPArchiveWithLevel(outputPath string, files map[string][]byte, level int) error {
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	archiveFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output archive: %w", err)
	}
	defer archiveFile.Close()

	zipWriter := zip.NewWriter(archiveFile)
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, level)
	})
	defer zipWriter.Close()

	for name, data := range files {
		entry, err := zipWriter.Create(name)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %w", name, err)
		}
		if _, err := entry.Write(data); err != nil {
			return fmt.Errorf("failed to write zip entry %s: %w", name, err)
		}
	}

	return nil
}

func writeSevenZipArchive(outputPath string, files map[string][]byte) error {
	return writeSevenZipArchiveWithLevel(outputPath, files, "-mx=5")
}

func writeSevenZipArchiveWithLevel(outputPath string, files map[string][]byte, level string) error {
	sevenZipPath, err := exec.LookPath("7z")
	if err != nil {
		sevenZipPath, err = exec.LookPath("7za")
	}
	if err != nil {
		return fmt.Errorf("7z tool not installed")
	}

	tempDir, err := os.MkdirTemp("", "archive-7z-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for name, data := range files {
		fullPath := filepath.Join(tempDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("failed to create temp path %s: %w", name, err)
		}
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write temp file %s: %w", name, err)
		}
	}

	cmd := exec.Command(sevenZipPath, "a", level, "-t7z", outputPath, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("7z command failed: %w", err)
	}

	return nil
}

func writeRarArchive(outputPath string, files map[string][]byte) error {
	return writeRarArchiveWithLevel(outputPath, files, "-m3")
}

func writeRarArchiveWithLevel(outputPath string, files map[string][]byte, level string) error {
	rarPath, err := exec.LookPath("rar")
	if err != nil {
		return fmt.Errorf("rar tool not installed")
	}

	tempDir, err := os.MkdirTemp("", "archive-rar-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for name, data := range files {
		fullPath := filepath.Join(tempDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("failed to create temp path %s: %w", name, err)
		}
		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write temp file %s: %w", name, err)
		}
	}

	cmd := exec.Command(rarPath, "a", level, outputPath, tempDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rar command failed: %w", err)
	}

	return nil
}

// RecommendCompression returns the most suitable compression strategy for a given file type.
func RecommendCompression(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".zip":
		return "ZIP: usually best for general-purpose packaging and for files that are already text-heavy or moderately compressed."
	case ".rar":
		return "RAR: often strong on solid compression and large binary data, but compatibility is more limited."
	case ".7z":
		return "7Z: often the best for maximum compression ratio on large archives, especially with solid blocks."
	case ".stl":
		return "STL: ZIP is the safest choice; it preserves the mesh file intact and is widely supported."
	case ".obj":
		return "OBJ: ZIP is the best default because OBJ often ships with companion MTL/texture files."
	case ".jpg", ".jpeg", ".png", ".webp":
		return "JPG/PNG/WebP: use the original format for best quality; ZIP only helps as a container, not as a better image codec."
	default:
		return "ZIP is the safest default for a single-file container; 7Z is often best when size matters most."
	}
}
