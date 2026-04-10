package web

import (
	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/config"
	"archive-duplicate-finder/internal/db"
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"archive-duplicate-finder/internal/similarity"
	"archive-duplicate-finder/internal/stl"
	"archive-duplicate-finder/internal/visual"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
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
	"github.com/nfnt/resize"
)

// Server represents the web dashboard server
type Server struct {
	addr               string
	report             *reporter.Report
	trashPath          string
	leaveRef           bool
	debug              bool
	runStep3Func       func()
	runVisualFunc      func()
	allFiles           []reporter.FileInfo
	cache              *db.Cache
	previewSem         chan struct{}
	scanDir            string
	config             *config.AppConfig
	mu                 sync.Mutex
	// activePreviewFiles tracks files currently being extracted for preview.
	// Key: file path (string), Value: *sync.WaitGroup
	activePreviewFiles sync.Map
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

	api.Get("/cache-stats", func(c *fiber.Ctx) error {
		size, _ := config.GetCacheSize()
		return c.JSON(fiber.Map{
			"size_bytes": size,
			"size_gb":    float64(size) / (1024 * 1024 * 1024),
			"limit_gb":   s.config.CacheLimitGB,
		})
	})

	api.Post("/cache-clear", func(c *fiber.Ctx) error {
		err := config.ClearCache()
		if s.cache != nil {
			_ = s.cache.ClearAllThumbnails()
		}
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendStatus(200)
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

		// Create a unique hash/filename for this specific file in the archive
		cacheKey := fmt.Sprintf("%x_%s", path, internalPath)
		cacheKey = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, cacheKey)

		// Determine actual cache path (STL rendering produces PNGs)
		isSTRRender := strings.ToLower(filepath.Ext(internalPath)) == ".stl" && c.Query("format") == "png"
		if isSTRRender {
			fileExt = ".png"
		}

		// 1. Try to get thumbnail from DB cache
		if s.cache != nil {
			if cachedData, cType, ok := s.cache.GetThumbnail(cacheKey); ok {
				c.Set("X-Internal-Path", internalPath)
				c.Set("Content-Type", cType)
				return c.Send(cachedData)
			}
		}

		// 2. Fallback to extracting it (limited concurrency)
		// Register this file as being actively extracted so that a concurrent
		// delete request can wait for us to finish before touching the file.
		wg := &sync.WaitGroup{}
		wg.Add(1)
		// Store the WaitGroup, or merge with an existing one if present.
		actual, loaded := s.activePreviewFiles.LoadOrStore(path, wg)
		if loaded {
			// Another goroutine already registered a WaitGroup; add to it.
			existingWg := actual.(*sync.WaitGroup)
			existingWg.Add(1)
			wg = existingWg
		}

		s.previewSem <- struct{}{}
		data, err := archive.GetFileFromArchive(path, internalPath)
		if err != nil {
			<-s.previewSem
			wg.Done()
			s.activePreviewFiles.Delete(path)
			return c.Status(404).SendString(err.Error())
		}

		// SPECIAL CASE: If it's an STL and we want a PNG preview
		if isSTRRender {
			info, err := stl.ParseSTL(data)
			if err == nil {
				pngData, err := stl.RenderToPNG(info, 800, 600)
				if err == nil {
					data = pngData
				}
			}
		}
		<-s.previewSem
		wg.Done()
		s.activePreviewFiles.Delete(path)

		contentType := getContentType(cacheKey + fileExt)
		
		// If it's an image, resize it to max 256x256
		if strings.HasPrefix(contentType, "image/") {
			img, _, err := image.Decode(bytes.NewReader(data))
			if err == nil {
				bounds := img.Bounds()
				if bounds.Dx() > 256 || bounds.Dy() > 256 {
					img = resize.Thumbnail(256, 256, img, resize.Bilinear)
					buf := new(bytes.Buffer)
					// Encode to JPEG to save space
					if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}); err == nil {
						data = buf.Bytes()
						contentType = "image/jpeg"
					}
				}
			}
		}

		if s.cache != nil {
			s.cache.PutThumbnail(cacheKey, data, contentType)
			// Enforce limit
			go s.checkCacheLimit()
		}

		c.Set("X-Internal-Path", internalPath)
		c.Set("Content-Type", contentType)
		return c.Send(data)
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

		// Wait for any in-progress preview extraction of this file to finish.
		// This prevents "file in use" errors on Windows when the archive reader
		// still has the file open while we try to rename/delete it.
		if v, ok := s.activePreviewFiles.Load(req.Path); ok {
			log.Printf("⏳ Waiting for active preview extraction of %s to finish before deleting...", req.Path)
			v.(*sync.WaitGroup).Wait()
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

	// Serve static dashboard files with index support
	app.Static("/", "./ui/out", fiber.Static{
		Index: "index.html",
	})

	// Final fallback for SPA routing: any non-API route that 404s should try to serve [path].html or index.html
	app.Use(func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api") {
			return c.Status(404).SendString("API route not found")
		}

		// Try to see if [path].html exists (e.g., /gallery -> gallery.html)
		htmlPath := filepath.Join("./ui/out", c.Path()+".html")
		if _, err := os.Stat(htmlPath); err == nil {
			return c.SendFile(htmlPath)
		}

		// Fallback to index.html for SPA client-side routing
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
					Type:        f.Type,
					ModTime:     f.ModTime,
					PHash:       f.PHash,
					VisualScore: f.VisualScore,
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

func (s *Server) checkCacheLimit() {
	if s.config == nil || s.config.CacheLimitGB <= 0 {
		return
	}

	limitBytes := int64(s.config.CacheLimitGB * 1024 * 1024 * 1024)

	// Clear db cache
	if s.cache != nil {
		_ = s.cache.CleanThumbnailsIfNeeded(limitBytes)
	}

	size, err := config.GetCacheSize()
	if err != nil {
		return
	}

	if size > limitBytes {
		log.Printf("🧹 Cache limit exceeded (%.2f GB > %.2f GB). Clearing cache...", float64(size)/(1024*1024*1024), s.config.CacheLimitGB)
		config.ClearCache()
		if s.cache != nil {
			_ = s.cache.ClearAllThumbnails()
		}
	}
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
