package main

/*
 * Archive Duplicate Finder
 * Created with the assistance of Antigravity (Google Deepmind AI)
 */

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"archive-duplicate-finder/internal/similarity"
	"archive-duplicate-finder/internal/stl"
)

type Config struct {
	Directory   string
	Threshold   int
	Mode        string
	Verbose     bool
	Recursive   bool
	OutputFile  string
	PDFFile     string
	DeleteMode  string // "oldest" or "contents"
	AutoDelete  bool
	Interactive bool
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Validate directory
	if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		log.Fatalf("❌ Directory does not exist: %s", config.Directory)
	}

	fmt.Printf("🔍 Archive Duplicate Finder\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	fmt.Printf("📂 Scanning directory: %s\n", config.Directory)
	fmt.Printf("🎯 Similarity threshold: %d%%\n", config.Threshold)
	fmt.Printf("🔧 Mode: %s\n", config.Mode)
	if config.DeleteMode != "" {
		fmt.Printf("🗑️  Cleanup Mode: %s (Auto: %v)\n", config.DeleteMode, config.AutoDelete)
	}
	fmt.Printf("\n")

	startTime := time.Now()

	// Step 1: Scan for archive files
	fmt.Println("📦 Step 1: Scanning for archive files...")
	files, err := scanner.ScanDirectory(config.Directory, config.Recursive)
	if err != nil {
		log.Fatalf("❌ Failed to scan directory: %v", err)
	}

	fmt.Printf("✅ Found %d archive files\n", len(files))
	scanner.PrintFileStats(files)
	fmt.Println()

	// Summary / Report Prep
	elapsed := time.Since(startTime)
	baseReport := reporter.Report{
		TotalFiles:       len(files),
		AnalysisDuration: elapsed.Seconds(),
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
	}

	// Step 2: Identical Size
	sizeGroups := scanner.GroupBySize(files)
	var finalSizeGroups []reporter.SizeGroup
	if config.Mode == "all" || config.Mode == "size" {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("🔄 Step 2: Analyzing identical sizes...")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		finalSizeGroups = analyzeSameSizeDifferentName(sizeGroups, config.Threshold, config.Verbose, config)

		if config.PDFFile != "" {
			report2 := baseReport
			report2.SizeGroups = finalSizeGroups
			pdfName := "Step2_Size_" + config.PDFFile
			fmt.Printf("\n📄 [BETA] Generating Step 2 PDF: %s\n", pdfName)
			reporter.ExportPDF(report2, pdfName)
		}
	}

	// Step 3: Similar Names
	if config.Mode == "all" || config.Mode == "name" {
		fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		if config.Interactive {
			fmt.Println("📝 Step 3: Similar name analysis (Interactive Mode)")
		} else {
			fmt.Println("📝 Step 3: Similar name analysis started in BACKGROUND...")
		}
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		runStep3 := func() []reporter.SimilarPair {
			similarPairs := similarity.FindSimilarNames(files, config.Threshold)
			finalSimilarPairs := analyzeSimilarNameDifferentSize(similarPairs, config.Verbose, config)

			if config.PDFFile != "" {
				report3 := baseReport
				report3.SimilarPairs = finalSimilarPairs
				report3.SizeGroups = finalSizeGroups // Include size groups in the final one too
				pdfName := "Final_Full_" + config.PDFFile
				reporter.ExportPDF(report3, pdfName)
				fmt.Printf("\n✅ Step 3 analysis FINISHED. Final PDF ready: %s\n", pdfName)
			}
			return finalSimilarPairs
		}

		if config.Interactive {
			// Run sequentially for interactivity to handle stdin correctly
			runStep3()
		} else {
			// Run in background
			done := make(chan bool)
			go func() {
				runStep3()
				done <- true
			}()

			fmt.Println("ℹ️  You can already check the Step 2 PDF while Step 3 works.")
			fmt.Println("   Press Ctrl+C to stop if you don't need the similarity analysis.")
			<-done
		}
	}

	elapsedTotal := time.Since(startTime)
	fmt.Printf("\n📈 Total processing time: %.2fs\n", elapsedTotal.Seconds())
}

func parseFlags() Config {
	config := Config{}

	flag.StringVar(&config.Directory, "dir", ".", "Directory to scan for archive files")
	flag.IntVar(&config.Threshold, "threshold", 70, "Similarity threshold percentage (0-100)")
	flag.StringVar(&config.Mode, "mode", "all", "Analysis mode: 'all', 'size', or 'name'")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&config.Recursive, "recursive", true, "Scan subdirectories recursively")
	flag.StringVar(&config.OutputFile, "json", "", "Output JSON file path")
	flag.StringVar(&config.PDFFile, "pdf", "", "Output PDF report path")
	flag.StringVar(&config.DeleteMode, "delete", "", "Cleanup mode: 'oldest' or 'contents'")
	flag.BoolVar(&config.AutoDelete, "yes", false, "Auto-confirm deletion without asking")
	flag.BoolVar(&config.Interactive, "interactive", false, "Choose which file to delete manually")

	flag.Parse()

	// Validate threshold
	if config.Threshold < 0 || config.Threshold > 100 {
		log.Fatal("❌ Threshold must be between 0 and 100")
	}

	// Validate mode
	if config.Mode != "all" && config.Mode != "size" && config.Mode != "name" {
		log.Fatal("❌ Mode must be 'all', 'size', or 'name'")
	}

	// Validate delete mode
	if config.DeleteMode != "" && config.DeleteMode != "oldest" && config.DeleteMode != "contents" {
		log.Fatal("❌ Delete mode must be 'oldest' or 'contents'")
	}

	return config
}

func analyzeSameSizeDifferentName(sizeGroups map[int64][]scanner.ArchiveFile, threshold int, verbose bool, config Config) []reporter.SizeGroup {
	var results []reporter.SizeGroup
	groupCount := 0
	totalFiles := 0

	for size, group := range sizeGroups {
		if len(group) < 2 {
			continue // Skip groups with only one file
		}

		groupCount++
		totalFiles += len(group)

		fmt.Printf("📦 Group %d (Size: %s)\n", groupCount, formatBytes(size))

		var currentGroup reporter.SizeGroup
		currentGroup.Size = size

		// Compare all pairs in the group
		for i := 0; i < len(group); i++ {
			f := group[i]
			currentGroup.Files = append(currentGroup.Files, reporter.FileInfo{
				Name: f.Name,
				Path: f.Path,
				Size: f.Size,
				Type: f.Type,
			})

			for j := i + 1; j < len(group); j++ {
				file1 := group[i]
				file2 := group[j]

				// Calculate name similarity
				sim := similarity.CalculateNameSimilarity(file1.Name, file2.Name)

				if sim >= float64(threshold) {
					fmt.Printf("  📄 %s (Mod: %v)\n", file1.Name, file1.ModTime.Format("2006-01-02 15:04"))
					fmt.Printf("  📄 %s (Mod: %v)\n", file2.Name, file2.ModTime.Format("2006-01-02 15:04"))
					fmt.Printf("  📊 Name similarity: %.1f%%\n", sim)

					if sim > 90 {
						fmt.Println("  ⚠️  HIGH PROBABILITY: Likely renamed duplicate")
					} else if sim > 75 {
						fmt.Println("  ⚠️  MEDIUM PROBABILITY: Possible variant or version")
					}

					// Cleanup logic
					if config.DeleteMode != "" || config.Interactive {
						handleCleanup(file1, file2, config)
					}

					fmt.Println()
				}
			}
		}
		results = append(results, currentGroup)
	}

	if groupCount == 0 {
		fmt.Println("✅ No files with identical size and different names found")
	} else {
		fmt.Printf("📊 Found %d groups with %d total files\n", groupCount, totalFiles)
	}
	return results
}

func analyzeSimilarNameDifferentSize(pairs []similarity.SimilarPair, verbose bool, config Config) []reporter.SimilarPair {
	var results []reporter.SimilarPair
	if len(pairs) == 0 {
		fmt.Println("✅ No files with similar names and different sizes found")
		return results
	}

	for i, pair := range pairs {
		fmt.Printf("🔍 Comparison %d: %s ↔ %s\n", i+1, pair.File1.Name, pair.File2.Name)
		fmt.Printf("  📊 Name similarity: %.1f%%\n", pair.Similarity)
		fmt.Printf("  📏 Size: %s ↔ %s", formatBytes(pair.File1.Size), formatBytes(pair.File2.Size))

		results = append(results, reporter.SimilarPair{
			File1: reporter.FileInfo{
				Name: pair.File1.Name,
				Path: pair.File1.Path,
				Size: pair.File1.Size,
				Type: pair.File1.Type,
			},
			File2: reporter.FileInfo{
				Name: pair.File2.Name,
				Path: pair.File2.Path,
				Size: pair.File2.Size,
				Type: pair.File2.Type,
			},
			Similarity: pair.Similarity,
		})

		sizeDiff := pair.File2.Size - pair.File1.Size
		if sizeDiff > 0 {
			fmt.Printf(" (+%s)\n", formatBytes(sizeDiff))
		} else {
			fmt.Printf(" (-%s)\n", formatBytes(-sizeDiff))
		}

		// Cleanup logic
		if config.DeleteMode != "" || config.Interactive {
			// For similar name/different size, we need to extract to see "which contains less"
			if config.DeleteMode == "contents" || (config.Interactive && config.DeleteMode == "contents") {
				contents1, _ := archive.ExtractArchive(pair.File1.Path)
				contents2, _ := archive.ExtractArchive(pair.File2.Path)
				pair.File1.FileCount = len(contents1)
				pair.File2.FileCount = len(contents2)
			}
			handleCleanup(pair.File1, pair.File2, config)
		}

		// Extract and compare contents (only if verbose mode is on)
		if verbose {
			fmt.Println("\n  📦 Extracting archives...")

			contents1, err1 := archive.ExtractArchive(pair.File1.Path)
			contents2, err2 := archive.ExtractArchive(pair.File2.Path)

			if err1 != nil || err2 != nil {
				fmt.Printf("  ❌ Error extracting archives: %v / %v\n", err1, err2)
				fmt.Println()
				continue
			}

			fmt.Printf("  ✅ Archive 1: %d files\n", len(contents1))
			fmt.Printf("  ✅ Archive 2: %d files\n", len(contents2))

			// Compare STL files
			fmt.Println("\n  🔬 Comparing STL contents:")
			compareSTLContents(contents1, contents2, verbose)
		}

		fmt.Println()
	}

	fmt.Printf("📊 Analyzed %d similar file pairs\n", len(pairs))
	return results
}

func compareSTLContents(contents1, contents2 map[string][]byte, verbose bool) {
	// Find common files
	allFiles := make(map[string]bool)
	for name := range contents1 {
		allFiles[name] = true
	}
	for name := range contents2 {
		allFiles[name] = true
	}

	for filename := range allFiles {
		data1, exists1 := contents1[filename]
		data2, exists2 := contents2[filename]

		if !exists1 {
			fmt.Printf("    ❌ %s - ONLY IN ARCHIVE 2\n", filename)
			continue
		}

		if !exists2 {
			fmt.Printf("    ❌ %s - ONLY IN ARCHIVE 1\n", filename)
			continue
		}

		// Check if it's an STL file
		if !stl.IsSTLFile(filename) {
			if verbose {
				fmt.Printf("    ℹ️  %s - Not an STL file (skipped)\n", filename)
			}
			continue
		}

		// Compare STL files
		identical, diff := stl.CompareSTL(data1, data2)

		if identical {
			fmt.Printf("    ✅ %s - IDENTICAL\n", filename)
		} else {
			fmt.Printf("    ⚠️  %s - MODIFIED\n", filename)
			if verbose && diff != nil {
				fmt.Printf("       • Vertices: %d → %d (%+d)\n",
					diff.Vertices1, diff.Vertices2, diff.Vertices2-diff.Vertices1)
				fmt.Printf("       • Triangles: %d → %d (%+d)\n",
					diff.Triangles1, diff.Triangles2, diff.Triangles2-diff.Triangles1)
				if diff.Description != "" {
					fmt.Printf("       • Changes: %s\n", diff.Description)
				}
			}
		}
	}
}

func handleCleanup(f1, f2 scanner.ArchiveFile, config Config) {
	// Skip if either file is a multi-volume part (part1, part2, etc.)
	if isMultiVolumePart(f1.Name) || isMultiVolumePart(f2.Name) {
		if config.Verbose {
			fmt.Printf("  ℹ️  Skipping cleanup: Multi-volume parts detected (%s or %s)\n", f1.Name, f2.Name)
		}
		return
	}

	if config.Interactive {
		fmt.Printf("  🤔 Interactive choice Required:\n")
		fmt.Printf("     [1] Delete: %s (%s, %v)\n", f1.Name, formatBytes(f1.Size), f1.ModTime.Format("2006-01-02"))
		fmt.Printf("     [2] Delete: %s (%s, %v)\n", f2.Name, formatBytes(f2.Size), f2.ModTime.Format("2006-01-02"))
		fmt.Printf("     [k] Keep both files\n")
		fmt.Printf("     Choice (1/2/k): ")

		var choice string
		fmt.Scanln(&choice)
		switch strings.ToLower(choice) {
		case "1":
			deleteFile(f1.Path)
		case "2":
			deleteFile(f2.Path)
		case "k":
			fmt.Println("     ✅ Keeping both files.")
		default:
			fmt.Println("     ⏭️  Skipping (invalid choice)")
		}
		return
	}

	var toDelete scanner.ArchiveFile
	var reason string

	if config.DeleteMode == "oldest" {
		if f1.ModTime.Before(f2.ModTime) {
			toDelete = f1
			reason = fmt.Sprintf("is older (%v < %v)", f1.ModTime.Format("2006-01-02"), f2.ModTime.Format("2006-01-02"))
		} else if f2.ModTime.Before(f1.ModTime) {
			toDelete = f2
			reason = fmt.Sprintf("is older (%v < %v)", f2.ModTime.Format("2006-01-02"), f1.ModTime.Format("2006-01-02"))
		}
	} else if config.DeleteMode == "contents" {
		// Least contents: smaller FileCount or smaller Size
		if f1.FileCount > 0 && f2.FileCount > 0 {
			if f1.FileCount < f2.FileCount {
				toDelete = f1
				reason = fmt.Sprintf("contains fewer files (%d < %d)", f1.FileCount, f2.FileCount)
			} else if f2.FileCount < f1.FileCount {
				toDelete = f2
				reason = fmt.Sprintf("contains fewer files (%d < %d)", f2.FileCount, f1.FileCount)
			}
		}

		// If FileCount is same or not available, check size
		if toDelete.Path == "" {
			if f1.Size < f2.Size {
				toDelete = f1
				reason = fmt.Sprintf("is smaller (%s < %s)", formatBytes(f1.Size), formatBytes(f2.Size))
			} else if f2.Size < f1.Size {
				toDelete = f2
				reason = fmt.Sprintf("is smaller (%s < %s)", formatBytes(f2.Size), formatBytes(f1.Size))
			}
		}
	}

	if toDelete.Path == "" {
		fmt.Println("  ℹ️  No clear candidate for deletion.")
		return
	}

	fmt.Printf("  🗑️  Candidate for deletion: %s (%s)\n", toDelete.Name, reason)

	if config.AutoDelete {
		deleteFile(toDelete.Path)
	} else {
		fmt.Printf("     Delete this file? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			deleteFile(toDelete.Path)
		}
	}
}

func deleteFile(path string) {
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("     ❌ Error deleting file: %v\n", err)
	} else {
		fmt.Println("     ✅ File deleted successfully.")
	}
}

func isMultiVolumePart(filename string) bool {
	filename = strings.ToLower(filename)

	// Common patterns: .part1.rar, .z01, .001
	if strings.Contains(filename, ".part") || strings.Contains(filename, ".z0") {
		return true
	}

	// Check for .001, .002 etc extensions
	ext := filepath.Ext(filename)
	if len(ext) == 4 && ext[0] == '.' {
		isNumeric := true
		for i := 1; i < 4; i++ {
			if ext[i] < '0' || ext[i] > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			return true
		}
	}

	return false
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
