package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRecompressFileCreatesZipArchive(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "model.stl")
	if err := os.WriteFile(inputPath, []byte("solid test\nfacet normal 0 0 1\n"), 0o644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "model_recompressed.zip")
	createdPath, err := RecompressFile(inputPath, outputPath)
	if err != nil {
		t.Fatalf("RecompressFile returned error: %v", err)
	}

	if createdPath != outputPath {
		t.Fatalf("expected output path %s, got %s", outputPath, createdPath)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected archive to exist: %v", err)
	}

	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("failed to open zip archive: %v", err)
	}
	defer reader.Close()

	if len(reader.File) != 1 {
		t.Fatalf("expected 1 file in archive, got %d", len(reader.File))
	}

	if reader.File[0].Name != filepath.Base(inputPath) {
		t.Fatalf("expected archived file %s, got %s", filepath.Base(inputPath), reader.File[0].Name)
	}
}

func TestRecompressArchiveRepackagesZipContents(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "source.zip")
	outputPath := filepath.Join(tempDir, "repacked.zip")

	createArchive := func(path string) error {
		archiveFile, err := os.Create(path)
		if err != nil {
			return err
		}
		defer archiveFile.Close()

		writer := zip.NewWriter(archiveFile)
		defer writer.Close()

		fileWriter, err := writer.Create("payload.txt")
		if err != nil {
			return err
		}
		_, err = fileWriter.Write([]byte("hello world from archive"))
		return err
	}

	if err := createArchive(inputPath); err != nil {
		t.Fatalf("failed to create source archive: %v", err)
	}

	createdPath, err := RecompressArchive(inputPath, outputPath)
	if err != nil {
		t.Fatalf("RecompressArchive returned error: %v", err)
	}

	if createdPath != outputPath {
		t.Fatalf("expected output path %s, got %s", outputPath, createdPath)
	}

	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("failed to open repacked archive: %v", err)
	}
	defer reader.Close()

	if len(reader.File) != 1 {
		t.Fatalf("expected 1 file in repacked archive, got %d", len(reader.File))
	}
}
