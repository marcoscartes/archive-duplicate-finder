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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"archive-duplicate-finder/internal/config"
	"archive-duplicate-finder/internal/db"
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"archive-duplicate-finder/internal/similarity"
	"archive-duplicate-finder/internal/stl"
	"archive-duplicate-finder/internal/visual"
	"archive-duplicate-finder/internal/web"
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
	TrashPath   string // Folder to move duplicates to
	LeaveRef    bool   // Leave a .txt link to the original
	Web         bool   // Start web dashboard
	Port        int    // Web server port
	Debug       bool   // Enable detailed debug logging
	RunStep3    bool   // Explicitly run Step 3 (Similarity Check)
	Version     bool   // Show version and exit
	Info        bool   // Show author and info and exit
}

func main() {
	// 1. Load Persistent Config
	appConfig, _ := config.LoadConfig()

	// 2. Parse command line flags (can override appConfig)
	flagConfig := parseFlags()

	// Configure logger with timestamps
	log.SetFlags(log.Ldate | log.Ltime)

	// Determine if we should run a CLI scan immediately
	isExplicitScan := false
	visitCount := 0
	flag.Visit(func(f *flag.Flag) {
		visitCount++
		if f.Name == "dir" {
			isExplicitScan = true
		}
	})

	// If no flags at all and no saved directory, we MUST start in web setup mode
	if visitCount == 0 && appConfig.Directory == "" {
		log.Println("🌐 No configuration found. Starting web setup mode...")
		startWebServer(flagConfig, nil, nil, nil, appConfig, nil, nil)
		// Block indefinitely
		select {}
	}

	// If no flags but we HAVE a saved config, load it into flagConfig and start web
	if visitCount == 0 && appConfig.Directory != "" {
		log.Printf("📂 Loading saved configuration: %s", appConfig.Directory)
		flagConfig.Directory = appConfig.Directory
		flagConfig.TrashPath = appConfig.TrashPath
		flagConfig.Threshold = appConfig.Threshold
		flagConfig.Recursive = appConfig.Recursive
		flagConfig.LeaveRef = appConfig.LeaveRef
		flagConfig.Web = true // Default to web if launched without args
	}

	// Validate directory
	if _, err := os.Stat(flagConfig.Directory); os.IsNotExist(err) {
		if isExplicitScan {
			log.Fatalf("❌ Directory does not exist: %s", flagConfig.Directory)
		} else {
			log.Printf("⚠️ Saved directory no longer exists: %s. Starting web setup...", flagConfig.Directory)
			startWebServer(flagConfig, nil, nil, nil, appConfig, nil, nil)
			// Block indefinitely
			select {}
		}
	}

	log.Printf("🔍 Archive Duplicate Finder")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	log.Printf("📂 Scanning directory: %s", flagConfig.Directory)
	log.Printf("🎯 Similarity threshold: %d%%", flagConfig.Threshold)
	log.Printf("🔧 Mode: %s", flagConfig.Mode)
	if flagConfig.Debug {
		log.Printf("🐛 DEBUG MODE: Enabled (Detailed Tracing)")
	}
	if flagConfig.DeleteMode != "" {
		log.Printf("🗑️  Cleanup Mode: %s (Auto: %v)", flagConfig.DeleteMode, flagConfig.AutoDelete)
	}
	fmt.Printf("\n")

	startTime := time.Now()

	// Step 1: Scan for archive files
	log.Println("📦 Step 1: Scanning for archive files...")
	files, err := scanner.ScanDirectory(flagConfig.Directory, flagConfig.Recursive)
	if err != nil {
		log.Fatalf("❌ Failed to scan directory: %v", err)
	}

	log.Printf("✅ Found %d archive files", len(files))
	scanner.PrintFileStats(files)
	fmt.Println()

	// Summary / Report Prep
	elapsed := time.Since(startTime)
	baseReport := reporter.Report{
		TotalFiles:       len(files),
		AnalysisDuration: elapsed.Seconds(),
		Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
		Status:           "analyzing",
	}

	// Initialize Cache
	cache, err := db.NewCache()
	// var fingerprint string
	if err != nil {
		log.Printf("⚠️  Could not initialize cache: %v", err)
	} else {
		defer cache.Close()
		// fingerprint = cache.CalculateFingerprint(files)
	}

	// Step 2: Identical Size
	sizeGroups := scanner.GroupBySize(files)
	var finalSizeGroups []reporter.SizeGroup
	if flagConfig.Mode == "all" || flagConfig.Mode == "size" {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("🔄 Step 2: Analyzing identical sizes...")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		finalSizeGroups = analyzeSameSizeDifferentName(sizeGroups, flagConfig.Threshold, flagConfig.Verbose, flagConfig)

		if flagConfig.PDFFile != "" {
			report2 := baseReport
			report2.SizeGroups = finalSizeGroups
			pdfName := "Step2_Size_" + flagConfig.PDFFile
			fmt.Printf("\n📄 [BETA] Generating Step 2 PDF: %s\n", pdfName)
			reporter.ExportPDF(report2, pdfName)
		}
	}

	// Build initial report for web (will be updated)
	finalReport := &baseReport
	finalReport.SizeGroups = finalSizeGroups

	var runStep3Trigger func()
	var runVisualTrigger func()

	// Step 3 Logic
	var finalSimilarGroups []reporter.SimilarityGroup
	runStep3Job := func() []reporter.SimilarityGroup {
		log.Printf("🚀 Optimized Clustering Engine: Active (O(N) Speed)")

		onProgress := func(p float64) {
			finalReport.Progress = p
			if !flagConfig.Web {
				fmt.Printf("\r🔍 Similar Names: [%-20s] %.1f%%", strings.Repeat("=", int(p/5)), p)
			}
		}

		simGroups := similarity.FindSimilarGroups(files, flagConfig.Threshold, flagConfig.Debug, onProgress)

		if !flagConfig.Web {
			fmt.Println()
		}

		var results []reporter.SimilarityGroup
		for _, g := range simGroups {
			var fileInfos []reporter.FileInfo
			for _, f := range g.Files {
				fileInfos = append(fileInfos, reporter.FileInfo{
					Name:    f.Name,
					Path:    f.Path,
					Size:    f.Size,
					Type:    f.Type,
					ModTime: f.ModTime.Format(time.RFC3339),
				})
			}
			results = append(results, reporter.SimilarityGroup{
				BaseName: g.BaseName,
				Files:    fileInfos,
			})
		}
		return results
	}

	runStep3Trigger = func() {
		if finalReport.Status == "analyzing_step3" {
			log.Println("ℹ️  Step 3 is already running.")
			return
		}

		log.Println("📝 Step 3: Similar name analysis STARTED (Clustering Mode)...")
		step3Start := time.Now()
		finalReport.Status = "analyzing_step3"
		finalReport.Progress = 0

		results := runStep3Job()

		finalReport.SimilarGroups = results
		finalReport.SimilarCount = len(results)
		finalReport.AnalysisDuration += time.Since(step3Start).Seconds()
		finalReport.Status = "finished"

		log.Printf("✅ Step 3 analysis FINISHED. Found %d similarity clusters.", len(results))

		if !flagConfig.Web {
			for i, g := range results {
				if i >= 10 && !flagConfig.Verbose {
					if i == 10 {
						fmt.Println("... (Use --verbose to see all groups)")
					}
					continue
				}
				fmt.Printf("🔍 Cluster: '%s' (%d files)\n", g.BaseName, len(g.Files))
				for _, f := range g.Files {
					fmt.Printf("  • %s (%s)\n", f.Name, formatBytes(f.Size))
				}
				fmt.Println()
			}
		}
	}

	runVisualTrigger = func() {
		if finalReport.Status == "analyzing_visual" {
			log.Println("ℹ️  Visual analysis is already running.")
			return
		}

		log.Println("🎨 Step 4: Visual Fingerprinting STARTED (Incremental Mode)...")
		finalReport.Status = "analyzing_visual"
		finalReport.Progress = 0

		hashDone := make(chan bool)
		go func() {
			onVisualProgress := func(p float64) {
				finalReport.Progress = p
				if !flagConfig.Web {
					fmt.Printf("\r🌆 Visual Hashing: [%-20s] %.1f%%",
						strings.Repeat("=", int(p/5)), p)
				}
			}
			visual.ProcessVisualHashes(files, cache, flagConfig.Debug, onVisualProgress)
			hashDone <- true
		}()

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		updateVisualGroups := func() {
			visualGroups := visual.FindVisualDuplicates(files, cache, flagConfig.Threshold)
			var reporterVisualGroups []reporter.SimilarityGroup
			for _, vg := range visualGroups {
				var fileInfos []reporter.FileInfo
				for _, f := range vg.Files {
					fileInfos = append(fileInfos, reporter.FileInfo{
						Name:    f.Name,
						Path:    f.Path,
						Size:    f.Size,
						Type:    f.Type,
						ModTime: f.ModTime,
						PHash:   f.PHash,
					})
				}
				reporterVisualGroups = append(reporterVisualGroups, reporter.SimilarityGroup{
					BaseName: vg.BaseName,
					Files:    fileInfos,
				})
			}
			finalReport.VisualGroups = reporterVisualGroups
			finalReport.VisualCount = len(reporterVisualGroups)
		}

	loop:
		for {
			select {
			case <-hashDone:
				if !flagConfig.Web {
					fmt.Println()
				}
				updateVisualGroups()
				break loop
			case <-ticker.C:
				updateVisualGroups()
			}
		}

		finalReport.Status = "finished"
		log.Printf("✅ Visual analysis FINISHED. Found %d visual duplicate groups total.", finalReport.VisualCount)
	}

	if flagConfig.Mode == "all" || flagConfig.Mode == "name" {
		if flagConfig.Interactive {
			// Interactive mode force
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("📝 Step 3: Similar name analysis (Interactive Mode)")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			finalSimilarGroups = runStep3Job()
			finalReport.SimilarGroups = finalSimilarGroups
			finalReport.SimilarCount = len(finalSimilarGroups)
			finalReport.Status = "finished"
		} else {
			// Background / On-Demand Mode
			if flagConfig.RunStep3 {
				fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				if flagConfig.Web {
					log.Println("📝 Step 3: Similar name analysis started in BACKGROUND...")
					fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					go runStep3Trigger()
					fmt.Println("ℹ️  You can check the dashboard while Step 3 works.")
				} else {
					log.Println("📝 Step 3: Similar name analysis started...")
					fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
					runStep3Trigger()
				}
			} else {
				log.Println("ℹ️  Step 3 (Similarity Check) skipped. Use --check-similar or Dashboard to run it.")
			}
		}
	} else {
		finalReport.Status = "finished"
	}

	// Start web dashboard
	if flagConfig.Web {
		// Convert scanner.ArchiveFile to reporter.FileInfo for the dashboard
		var allFileInfos []reporter.FileInfo
		for _, f := range files {
			allFileInfos = append(allFileInfos, reporter.FileInfo{
				Name:    f.Name,
				Path:    f.Path,
				Size:    f.Size,
				Type:    f.Type,
				ModTime: f.ModTime.Format(time.RFC3339),
			})
		}

		startWebServer(flagConfig, finalReport, allFileInfos, cache, appConfig, runStep3Trigger, runVisualTrigger)
	}

	elapsedTotal := time.Since(startTime)
	log.Printf("📈 Total processing time: %.2fs", elapsedTotal.Seconds())

	// If web server is running, block indefinitely
	if flagConfig.Web {
		log.Println("📡 Dashboard is ACTIVE. Press Ctrl+C to shutdown.")
		select {}
	}
}

func startWebServer(config Config, report *reporter.Report, allFiles []reporter.FileInfo, cache *db.Cache, appConfig *config.AppConfig, runStep3 func(), runVisual func()) {
	// Set triggers for on-demand analysis if needed
	srv := web.NewServer(config.Port, report, config.TrashPath, config.LeaveRef, runStep3, runVisual, allFiles, cache, config.Directory, appConfig)
	srv.SetDebug(config.Debug)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("❌ Web server error: %v", err)
		}
	}()

	// Auto-open browser
	go func() {
		time.Sleep(1 * time.Second) // Give server a moment to bind
		url := fmt.Sprintf("http://localhost:%d", config.Port)
		log.Printf("🌍 Opening dashboard at %s ...", url)
		openBrowser(url)
	}()
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
	flag.StringVar(&config.TrashPath, "trash", "", "Folder to move duplicates to (instead of deleting)")
	flag.BoolVar(&config.LeaveRef, "ref", false, "Leave a .txt file pointing to the preserved original")
	flag.BoolVar(&config.Web, "web", false, "Start web dashboard after analysis")
	flag.IntVar(&config.Port, "port", 8080, "Web server port")
	flag.BoolVar(&config.Debug, "debug", false, "Enable detailed debug logging for troubleshooting")
	flag.BoolVar(&config.RunStep3, "check-similar", false, "Explicitly run Step 3 (Similarity Check). Default is on-demand.")
	flag.BoolVar(&config.Version, "version", false, "Show version information and exit")
	flag.BoolVar(&config.Info, "info", false, "Show project information, author and license")

	flag.Parse()

	if config.Version {
		fmt.Println("Archive Duplicate Finder v1.8.0")
		os.Exit(0)
	}

	if config.Info {
		fmt.Println("📦 Archive Duplicate Finder")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("👤 Author: Marcos Cartes")
		fmt.Println("🤖 Co-Author: Antigravity (Google Deepmind AI)")
		fmt.Println("🌐 GitHub: https://github.com/marcoscartes/archive-duplicate-finder")
		fmt.Println("📄 License: MIT")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		os.Exit(0)
	}

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
				Name:    f.Name,
				Path:    f.Path,
				Size:    f.Size,
				Type:    f.Type,
				ModTime: f.ModTime.Format(time.RFC3339),
			})

			for j := i + 1; j < len(group); j++ {
				file1 := group[i]
				file2 := group[j]

				// Calculate name similarity
				sim := similarity.CalculateNameSimilarity(file1.Name, file2.Name, config.Debug)

				// Skip if they are different parts of the same multi-volume set
				is1, base1, p1 := file1.IsMultiVolumePart()
				is2, base2, p2 := file2.IsMultiVolumePart()
				if is1 && is2 && base1 == base2 && p1 != p2 {
					if verbose {
						fmt.Printf("  ⏩ Skipping multi-volume set parts: %s vs %s\n", file1.Name, file2.Name)
					}
					continue
				}

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
			performFileAction(f1, f2, config)
		case "2":
			performFileAction(f2, f1, config)
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

	// Identify preserved file
	preserved := f1
	if toDelete.Path == f1.Path {
		preserved = f2
	}

	if config.AutoDelete {
		performFileAction(toDelete, preserved, config)
	} else {
		fmt.Printf("     Delete/Move this file? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			performFileAction(toDelete, preserved, config)
		}
	}
}

func performFileAction(target, preserved scanner.ArchiveFile, config Config) {
	if config.TrashPath != "" {
		// Ensure trash directory exists
		if _, err := os.Stat(config.TrashPath); os.IsNotExist(err) {
			os.MkdirAll(config.TrashPath, 0755)
		}

		destPath := filepath.Join(config.TrashPath, target.Name)
		err := os.Rename(target.Path, destPath)
		if err != nil {
			fmt.Printf("     ❌ Error moving to trash: %v (Attempting delete instead)\n", err)
			deleteFile(target.Path)
		} else {
			fmt.Printf("     ✅ Moved to trash: %s\n", destPath)
		}
	} else {
		deleteFile(target.Path)
	}

	// Create reference link if requested
	if config.LeaveRef {
		refPath := target.Path + ".duplicate.txt"
		content := fmt.Sprintf("Archive Duplicate Finder\n-----------------------\nAction: Removed as duplicate\nDate: %s\nOriginal kept: %s\nOriginal size: %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			preserved.Path,
			formatBytes(preserved.Size))

		err := os.WriteFile(refPath, []byte(content), 0644)
		if err != nil {
			fmt.Printf("     ⚠️  Could not create reference file: %v\n", err)
		} else {
			fmt.Printf("     📝 Reference note created: %s\n", filepath.Base(refPath))
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

// openBrowser opens the specified URL in the default browser of the user.
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("⚠️  Could not open browser: %v", err)
	}
}
