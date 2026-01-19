package web

import (
	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/config"
	"archive-duplicate-finder/internal/db"
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"archive-duplicate-finder/internal/similarity"
	"archive-duplicate-finder/internal/visual"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

// Server represents the web dashboard server
type Server struct {
	addr          string
	report        *reporter.Report
	trashPath     string
	leaveRef      bool
	debug         bool
	runStep3Func  func()
	runVisualFunc func()
	allFiles      []reporter.FileInfo
	cache         *db.Cache
	previewSem    chan struct{}
	scanDir       string
	config        *config.AppConfig
	mu            sync.Mutex
}

// NewServer creates a new web dashboard server
func NewServer(port int, report *reporter.Report, trashPath string, leaveRef bool, runStep3Func func(), runVisualFunc func(), allFiles []reporter.FileInfo, cache *db.Cache, scanDir string, appConfig *config.AppConfig) *Server {
	return &Server{
		addr:          fmt.Sprintf(":%d", port),
		report:        report,
		trashPath:     trashPath,
		leaveRef:      leaveRef,
		runStep3Func:  runStep3Func,
		runVisualFunc: runVisualFunc,
		allFiles:      allFileInfos(allFiles),
		cache:         cache,
		previewSem:    make(chan struct{}, 4), // Allow 4 concurrent extractions
		scanDir:       scanDir,
		config:        appConfig,
	}
}

func allFileInfos(files []reporter.FileInfo) []reporter.FileInfo {
	if files == nil {
		return []reporter.FileInfo{}
	}
	return files
}

// SetDebug enables or disables debug mode
func (s *Server) SetDebug(enabled bool) {
	s.debug = enabled
}

// Start starts the web server
func (s *Server) Start() error {
	app := fiber.New(fiber.Config{
		AppName: "Archive Duplicate Finder Dashboard",
	})

	// Enable CORS
	app.Use(cors.New())

	// Add detailed logging in debug mode
	if s.debug {
		app.Use(logger.New(logger.Config{
			Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
		}))
	}

	// API Routes
	api := app.Group("/api")

	api.Post("/run-step-3", func(c *fiber.Ctx) error {
		go s.RunStep3()
		return c.SendStatus(202)
	})

	api.Post("/run-visual", func(c *fiber.Ctx) error {
		go s.RunVisual()
		return c.SendStatus(202)
	})

	api.Post("/open-directory", func(c *fiber.Ctx) error {
		path := c.Query("path")
		if path == "" {
			path = s.scanDir
		}
		if path == "" && s.config != nil {
			path = s.config.Directory
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		log.Printf("📂 Opening directory in explorer: %s", absPath)

		var cmdErr error
		switch runtime.GOOS {
		case "linux":
			cmdErr = exec.Command("xdg-open", absPath).Start()
		case "windows":
			cmdErr = exec.Command("rundll32", "url.dll,FileProtocolHandler", absPath).Start()
		case "darwin":
			cmdErr = exec.Command("open", absPath).Start()
		default:
			cmdErr = fmt.Errorf("unsupported platform")
		}

		if cmdErr != nil {
			log.Printf("⚠️ Could not open directory: %v", cmdErr)
			return c.Status(500).SendString(cmdErr.Error())
		}
		return c.SendStatus(200)
	})

	api.Get("/config", func(c *fiber.Ctx) error {
		return c.JSON(s.config)
	})

	api.Post("/config", func(c *fiber.Ctx) error {
		var cfg config.AppConfig
		if err := c.BodyParser(&cfg); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		s.mu.Lock()
		s.config = &cfg
		s.scanDir = cfg.Directory
		s.trashPath = cfg.TrashPath
		s.leaveRef = cfg.LeaveRef
		s.mu.Unlock()

		if err := config.SaveConfig(&cfg); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
	})

	api.Post("/start-scan", func(c *fiber.Ctx) error {
		s.mu.Lock()
		if s.report != nil && (s.report.Status == "analyzing" || s.report.Status == "analyzing_step3" || s.report.Status == "analyzing_visual") {
			s.mu.Unlock()
			return c.Status(400).SendString("Scan already in progress")
		}

		cfg := s.config
		if cfg == nil {
			s.mu.Unlock()
			return c.Status(400).SendString("No configuration set")
		}
		s.mu.Unlock()

		go s.performFullScan(cfg)
		return c.SendStatus(202)
	})

	api.Post("/reset", func(c *fiber.Ctx) error {
		s.mu.Lock()
		s.report = nil
		s.allFiles = []reporter.FileInfo{}
		s.mu.Unlock()
		return c.SendStatus(200)
	})

	api.Get("/report", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.report == nil {
			return c.Status(200).JSON(fiber.Map{"status": "idle"})
		}

		// Filter out ignored groups
		var filteredSizeGroups []reporter.SizeGroup
		for _, g := range s.report.SizeGroups {
			if s.cache != nil && s.cache.IsGroupIgnored(g.Hash()) {
				continue
			}
			filteredSizeGroups = append(filteredSizeGroups, g)
		}

		var filteredSimilarGroups []reporter.SimilarityGroup
		for _, g := range s.report.SimilarGroups {
			if s.cache != nil && s.cache.IsGroupIgnored(g.Hash()) {
				continue
			}
			filteredSimilarGroups = append(filteredSimilarGroups, g)
		}

		var filteredVisualGroups []reporter.SimilarityGroup
		for _, g := range s.report.VisualGroups {
			if s.cache != nil && s.cache.IsGroupIgnored(g.Hash()) {
				continue
			}
			filteredVisualGroups = append(filteredVisualGroups, g)
		}

		reportCopy := *s.report
		reportCopy.SizeGroups = filteredSizeGroups
		reportCopy.SimilarGroups = filteredSimilarGroups
		reportCopy.VisualGroups = filteredVisualGroups

		if c.Query("exclude_similar") == "true" {
			reportCopy.SimilarGroups = nil
			return c.Status(200).JSON(reportCopy)
		}
		return c.Status(200).JSON(reportCopy)
	})

	api.Post("/mark-as-good", func(c *fiber.Ctx) error {
		type markRequest struct {
			Files []reporter.FileInfo `json:"files"`
		}
		var req markRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Invalid request body")
		}

		if len(req.Files) == 0 {
			return c.Status(400).SendString("No files provided")
		}

		hash := reporter.CalculateGroupHash(req.Files)
		log.Printf("👍 Marking group as good (ignored): %s", hash)

		if s.cache != nil {
			s.cache.AddIgnoredGroup(hash)
		}

		// Also remove it from memory immediately
		s.mu.Lock()
		defer s.mu.Unlock()

		// Helper to filter groups
		filterGroups := func(groups []reporter.SimilarityGroup) []reporter.SimilarityGroup {
			var filtered []reporter.SimilarityGroup
			for _, g := range groups {
				if g.Hash() != hash {
					filtered = append(filtered, g)
				}
			}
			return filtered
		}

		s.report.SimilarGroups = filterGroups(s.report.SimilarGroups)
		s.report.VisualGroups = filterGroups(s.report.VisualGroups)

		// Filter size groups separately
		var newSizeGroups []reporter.SizeGroup
		for _, g := range s.report.SizeGroups {
			if g.Hash() != hash {
				newSizeGroups = append(newSizeGroups, g)
			}
		}
		s.report.SizeGroups = newSizeGroups

		return c.SendStatus(200)
	})

	api.Get("/stats", func(c *fiber.Ctx) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.report == nil {
			return c.Status(200).JSON(fiber.Map{
				"totalFiles": 0,
				"duplicates": 0,
				"similar":    0,
				"duration":   0,
			})
		}
		return c.Status(200).JSON(fiber.Map{
			"totalFiles": s.report.TotalFiles,
			"duplicates": len(s.report.SizeGroups),
			"similar":    len(s.report.SimilarGroups),
			"duration":   s.report.AnalysisDuration,
		})
	})

	api.Get("/all-files", func(c *fiber.Ctx) error {
		// Use the full scanned list if available, otherwise fallback to map-based collection
		var files []reporter.FileInfo
		if len(s.allFiles) > 0 {
			files = s.allFiles
		} else {
			fileMap := make(map[string]reporter.FileInfo)
			for _, group := range s.report.SizeGroups {
				for _, file := range group.Files {
					fileMap[file.Path] = file
				}
			}
			for _, group := range s.report.SimilarGroups {
				for _, file := range group.Files {
					fileMap[file.Path] = file
				}
			}
			files = make([]reporter.FileInfo, 0, len(fileMap))
			for _, file := range fileMap {
				files = append(files, file)
			}
		}

		return c.Status(200).JSON(fiber.Map{
			"files": files,
			"total": len(files),
		})
	})

	// Endpoint: /api/preview?path=...&internal_path=...
	api.Get("/preview", func(c *fiber.Ctx) error {
		path := c.Query("path")
		internalPath := c.Query("internal_path")
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		// Determine if it's a direct file or an archive
		isArchive := false
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz" {
			isArchive = true
		}

		// 1. Handling when internalPath is NOT specified (Initial Gallery Load)
		if internalPath == "" {
			if !isArchive {
				// Direct file (image, video, model): Send with correct content type
				contentType := getContentType(path)
				c.Set("Content-Type", contentType)
				return c.SendFile(path)
			}

			// Check cache first
			info, _ := os.Stat(path)
			modTime := ""
			if info != nil {
				modTime = info.ModTime().String()
			}

			var found bool
			if s.cache != nil && c.Query("type") != "model" {
				internalPath, found = s.cache.GetPreviewPath(path, modTime)
			}

			if !found {
				// Archive without internal path: Find the best preview filename efficiently
				var filename string
				var err error

				if c.Query("type") == "model" {
					filename, err = archive.FindBestSTLInArchive(path)
				} else {
					filename, err = archive.FindPreviewPathInArchive(path)
				}

				if err != nil {
					return c.Status(404).SendString(err.Error())
				}
				internalPath = filename

				// Save to cache (only if standard preview)
				if s.cache != nil && c.Query("type") != "model" {
					s.cache.PutPreviewPath(path, internalPath, modTime)
				}
			}
		}

		// 2. Files inside archives (or found video preview from above)
		fileExt := strings.ToLower(filepath.Ext(internalPath))

		// For images, models or videos inside archives, use disk cache
		tempDir := filepath.Join(os.TempDir(), "archive-finder-cache")
		os.MkdirAll(tempDir, 0755)

		// Create a unique hash/filename for this specific file in the archive
		cacheKey := fmt.Sprintf("%x_%s", path, internalPath)
		cacheKey = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, cacheKey)

		cachePath := filepath.Join(tempDir, cacheKey+fileExt)

		// If not cached, extract it (limited concurrency)
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			s.previewSem <- struct{}{}
			data, err := archive.GetFileFromArchive(path, internalPath)
			if err != nil {
				<-s.previewSem
				return c.Status(404).SendString(err.Error())
			}
			os.WriteFile(cachePath, data, 0644)
			<-s.previewSem
		}

		c.Set("X-Internal-Path", internalPath)
		c.Set("Content-Type", getContentType(internalPath))
		return c.SendFile(cachePath)
	})

	api.Get("/list-previews", func(c *fiber.Ctx) error {
		path := c.Query("path")
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		previews, err := archive.ListPreviewsInArchive(path)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}

		return c.Status(200).JSON(fiber.Map{
			"previews": previews,
		})
	})

	api.Get("/open", func(c *fiber.Ctx) error {
		path := c.Query("path")
		mode := c.Query("mode", "reveal") // "reveal" or "launch"
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			if mode == "reveal" {
				cmd = exec.Command("explorer.exe", "/select,", path)
			} else {
				// Launch with associated app
				cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path)
			}
		case "darwin":
			if mode == "reveal" {
				cmd = exec.Command("open", "-R", path)
			} else {
				cmd = exec.Command("open", path)
			}
		case "linux":
			if mode == "reveal" {
				cmd = exec.Command("xdg-open", filepath.Dir(path))
			} else {
				cmd = exec.Command("xdg-open", path)
			}
		default:
			return c.Status(500).SendString("Unsupported OS")
		}

		if err := cmd.Start(); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
	})

	api.Post("/delete", func(c *fiber.Ctx) error {
		type deleteRequest struct {
			Path string `json:"path"`
		}
		var req deleteRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Invalid request body")
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		// 1. Perform FS action
		log.Printf("🗑️ Dashboard Request: Delete %s", req.Path)
		if s.trashPath != "" {
			if _, err := os.Stat(s.trashPath); os.IsNotExist(err) {
				os.MkdirAll(s.trashPath, 0755)
			}
			dest := filepath.Join(s.trashPath, filepath.Base(req.Path))
			log.Printf("📦 Moving to trash: %s -> %s", req.Path, dest)
			if err := os.Rename(req.Path, dest); err != nil {
				log.Printf("⚠️ Rename failed: %v. Trying Remove...", err)
				if err := os.Remove(req.Path); err != nil {
					log.Printf("❌ Delete failed: %v", err)
					return c.Status(500).SendString(err.Error())
				}
			}
			if s.leaveRef {
				refPath := req.Path + ".duplicate.txt"
				content := fmt.Sprintf("Archive Duplicate Finder\nOriginal kept: ... (Dashboard Action)\nDate: %s\n", time.Now().Format("2006-01-02 15:04:05"))
				_ = os.WriteFile(refPath, []byte(content), 0644)
			}
		} else {
			log.Printf("🔥 Permanently deleting: %s", req.Path)
			if err := os.Remove(req.Path); err != nil {
				log.Printf("❌ Delete failed: %v", err)
				return c.Status(500).SendString(err.Error())
			}
		}

		// 2. Remove from report and update stats
		s.report.TotalFiles--

		// Remove from Similarity Groups (Clusters)
		newGroups := make([]reporter.SimilarityGroup, 0)
		for _, g := range s.report.SimilarGroups {
			newFiles := make([]reporter.FileInfo, 0)
			for _, f := range g.Files {
				if f.Path != req.Path {
					newFiles = append(newFiles, f)
				}
			}
			// Keep group if it still has at least 2 files
			if len(newFiles) >= 2 {
				g.Files = newFiles
				newGroups = append(newGroups, g)
			}
		}
		s.report.SimilarGroups = newGroups
		s.report.SimilarCount = len(newGroups)

		// Remove from Size Groups
		var newSizeGroups []reporter.SizeGroup
		for i := range s.report.SizeGroups {
			newFiles := make([]reporter.FileInfo, 0)
			for _, f := range s.report.SizeGroups[i].Files {
				if f.Path != req.Path {
					newFiles = append(newFiles, f)
				}
			}
			// Only keep the group if it still has at least 2 files (a duplicate group)
			if len(newFiles) >= 2 {
				s.report.SizeGroups[i].Files = newFiles
				newSizeGroups = append(newSizeGroups, s.report.SizeGroups[i])
			}
		}
		s.report.SizeGroups = newSizeGroups

		log.Println("✅ Report state updated successfully")
		return c.SendStatus(200)
	})

	// Serve static dashboard files
	app.Static("/", "./ui/out")

	// Final fallback for SPA routing: any non-API route that 404s should serve index.html
	// This allows browser reloads on routes like /gallery to work correctly.
	app.Use(func(c *fiber.Ctx) error {
		// If it's an API route, return 404
		if strings.HasPrefix(c.Path(), "/api") {
			return c.Next()
		}
		// Otherwise serve index.html from static out
		return c.SendFile("./ui/out/index.html")
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("Archive Duplicate Finder Dashboard API is running")
	})

	log.Printf("🚀 Web Dashboard available at: http://localhost%s", s.addr)
	return app.Listen(s.addr)
}

func (s *Server) performFullScan(cfg *config.AppConfig) {
	log.Printf("🔍 Starting web-triggered scan: %s", cfg.Directory)
	s.mu.Lock()
	s.report = &reporter.Report{
		Status: "analyzing",
	}
	s.allFiles = []reporter.FileInfo{}
	s.mu.Unlock()

	startTime := time.Now()
	files, err := scanner.ScanDirectory(cfg.Directory, cfg.Recursive)
	if err != nil {
		log.Printf("❌ Scan failed: %v", err)
		s.mu.Lock()
		s.report.Status = "error"
		s.mu.Unlock()
		return
	}

	// Update allFiles for the gallery
	var allFiles []reporter.FileInfo
	for _, f := range files {
		allFiles = append(allFiles, reporter.FileInfo{
			Name:    f.Name,
			Path:    f.Path,
			Size:    f.Size,
			Type:    f.Type,
			ModTime: f.ModTime.Format(time.RFC3339),
		})
	}

	sizeGroups := scanner.GroupBySize(files)
	var finalSizeGroups []reporter.SizeGroup
	for size, group := range sizeGroups {
		if len(group) < 2 {
			continue
		}
		var currentGroup reporter.SizeGroup
		currentGroup.Size = size
		for _, f := range group {
			currentGroup.Files = append(currentGroup.Files, reporter.FileInfo{
				Name:    f.Name,
				Path:    f.Path,
				Size:    f.Size,
				Type:    f.Type,
				ModTime: f.ModTime.Format(time.RFC3339),
			})
		}
		finalSizeGroups = append(finalSizeGroups, currentGroup)
	}

	s.mu.Lock()
	s.report.TotalFiles = len(files)
	s.report.SizeGroups = finalSizeGroups
	s.report.AnalysisDuration = time.Since(startTime).Seconds()
	s.allFiles = allFiles
	s.report.Status = "finished"
	s.mu.Unlock()

	log.Printf("✅ Scan completed. Found %d files and %d size groups.", len(files), len(finalSizeGroups))

	// Trigger similarity automatically if configured? (Maybe later)
}

func (s *Server) RunStep3() {
	s.mu.Lock()
	if s.report == nil {
		s.mu.Unlock()
		return
	}
	if s.report.Status == "analyzing_step3" {
		s.mu.Unlock()
		return
	}
	s.report.Status = "analyzing_step3"
	s.report.Progress = 0
	scanDir := s.scanDir
	threshold := 70
	if s.config != nil {
		threshold = s.config.Threshold
	}
	s.mu.Unlock()

	log.Printf("📝 Web-triggered Step 3 analysis started...")
	startTime := time.Now()

	// Need scanner.ArchiveFile objects.
	files, _ := scanner.ScanDirectory(scanDir, true)

	onProgress := func(p float64) {
		s.mu.Lock()
		s.report.Progress = p
		s.mu.Unlock()
	}

	simGroups := similarity.FindSimilarGroups(files, threshold, s.debug, onProgress)

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

	s.mu.Lock()
	s.report.SimilarGroups = results
	s.report.SimilarCount = len(results)
	s.report.AnalysisDuration += time.Since(startTime).Seconds()
	s.report.Status = "finished"
	s.mu.Unlock()
	log.Printf("✅ Step 3 finished. Found %d clusters.", len(results))
}

func (s *Server) RunVisual() {
	s.mu.Lock()
	if s.report == nil || s.report.Status == "analyzing_visual" {
		s.mu.Unlock()
		return
	}
	s.report.Status = "analyzing_visual"
	s.report.Progress = 0
	scanDir := s.scanDir
	threshold := 70
	if s.config != nil {
		threshold = s.config.Threshold
	}
	s.mu.Unlock()

	log.Printf("🎨 Web-triggered Visual analysis started...")

	files, _ := scanner.ScanDirectory(scanDir, true)

	hashDone := make(chan bool)
	go func() {
		onVisualProgress := func(p float64) {
			s.mu.Lock()
			s.report.Progress = p
			s.mu.Unlock()
		}
		visual.ProcessVisualHashes(files, s.cache, s.debug, onVisualProgress)
		hashDone <- true
	}()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	updateVisualGroups := func() {
		visualGroups := visual.FindVisualDuplicates(files, s.cache, threshold)
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
		s.mu.Lock()
		s.report.VisualGroups = reporterVisualGroups
		s.report.VisualCount = len(reporterVisualGroups)
		s.mu.Unlock()
	}

loop:
	for {
		select {
		case <-hashDone:
			updateVisualGroups()
			break loop
		case <-ticker.C:
			updateVisualGroups()
		}
	}

	s.mu.Lock()
	s.report.Status = "finished"
	s.mu.Unlock()
	log.Printf("✅ Visual analysis finished.")
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".stl":
		return "model/stl"
	case ".obj":
		return "model/obj"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}
