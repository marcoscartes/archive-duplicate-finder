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
	// activePreviewFiles tracks files currently being extracted for preview.
	// Key: file path (string), Value: *sync.WaitGroup
	activePreviewFiles sync.Map
	// Background preview job queue and statuses
	jobQueue  chan *previewJob
	jobStatus sync.Map // jobID -> status string (queued|processing|done|failed)
	// recompressJobs keeps track of ongoing recompression tasks and their progress.
	recompressJobs   map[string]*recompressJob
	recompressJobsMu sync.Mutex
}

type recompressJob struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Status     string    `json:"status"`
	Percent    int       `json:"percent"`
	Message    string    `json:"message"`
	SizeBefore int64     `json:"size_before,omitempty"`
	SizeAfter  int64     `json:"size_after,omitempty"`
	Started    time.Time `json:"started_at"`
	Finished   time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// previewJob represents a background thumbnail rendering job
type previewJob struct {
	JobID       string
	Path        string
	Internal    string
	CacheKey    string
	FileExt     string
	ContentType string
	ModTime     string
	IsSTRRender bool
}

// NewServer creates a new web dashboard server
func NewServer(port int, report *reporter.Report, trashPath string, leaveRef bool, runStep3Func func(), runVisualFunc func(), allFiles []reporter.FileInfo, cache *db.Cache, scanDir string, appConfig *config.AppConfig) *Server {
	return &Server{
		addr:           fmt.Sprintf(":%d", port),
		report:         report,
		trashPath:      trashPath,
		leaveRef:       leaveRef,
		runStep3Func:   runStep3Func,
		runVisualFunc:  runVisualFunc,
		allFiles:       allFileInfos(allFiles),
		cache:          cache,
		previewSem:     make(chan struct{}, 16), // Allow 16 concurrent extractions (increased from 4)
		scanDir:        scanDir,
		config:         appConfig,
		recompressJobs: make(map[string]*recompressJob),
		jobQueue:       make(chan *previewJob, 128),
	}
}

func allFileInfos(files []reporter.FileInfo) []reporter.FileInfo {
	if files == nil {
		return []reporter.FileInfo{}
	}
	return files
}

func (s *Server) createRecompressJob(path string) string {
	s.recompressJobsMu.Lock()
	defer s.recompressJobsMu.Unlock()

	jobID := fmt.Sprintf("recompress-%d", time.Now().UnixNano())
	sizeBefore := int64(0)
	if info, err := os.Stat(path); err == nil {
		sizeBefore = info.Size()
	}
	s.recompressJobs[jobID] = &recompressJob{
		ID:         jobID,
		Path:       path,
		Status:     "queued",
		Percent:    0,
		Message:    "Queued",
		SizeBefore: sizeBefore,
		Started:    time.Now(),
	}
	log.Printf("📦 [API] Recompression queued: %s", path)
	return jobID
}

func (s *Server) getRecompressJob(jobID string) *recompressJob {
	s.recompressJobsMu.Lock()
	defer s.recompressJobsMu.Unlock()
	return s.recompressJobs[jobID]
}

func (s *Server) updateRecompressJob(jobID, status string, percent int, message string) {
	s.recompressJobsMu.Lock()
	defer s.recompressJobsMu.Unlock()
	job, ok := s.recompressJobs[jobID]
	if !ok || job == nil {
		return
	}
	job.Status = status
	job.Percent = percent
	job.Message = message
	log.Printf("📦 [API] Recompression %s: %d%% - %s", job.Path, percent, message)
}

func (s *Server) completeRecompressJob(jobID string, percent int, status, errMsg string, sizeAfter int64) {
	s.recompressJobsMu.Lock()
	defer s.recompressJobsMu.Unlock()
	job, ok := s.recompressJobs[jobID]
	if !ok || job == nil {
		return
	}

	normalizedStatus := strings.ToLower(status)
	if normalizedStatus == "finished" || normalizedStatus == "complete" || normalizedStatus == "completed" {
		normalizedStatus = "completed"
	} else if normalizedStatus == "failed" || normalizedStatus == "error" || normalizedStatus == "failure" {
		normalizedStatus = "failed"
	}

	job.Status = normalizedStatus
	job.Percent = percent
	job.SizeAfter = sizeAfter
	if normalizedStatus == "completed" {
		job.Message = fmt.Sprintf("Completed: %s → %s", formatBytes(job.SizeBefore), formatBytes(job.SizeAfter))
		job.Finished = time.Now()
		log.Printf("✅ [API] Recompression completed for %s: %s → %s", job.Path, formatBytes(job.SizeBefore), formatBytes(job.SizeAfter))
	} else {
		job.Message = "Failed"
		job.Finished = time.Now()
		job.Error = errMsg
		log.Printf("❌ [API] Recompression failed for %s: %s", job.Path, errMsg)
	}
}

// SetDebug enables or disables debug mode
func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
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
		log.Printf("📋 [API] Received request for all files from gallery")

		// Use the full scanned list if available, otherwise fallback to map-based collection
		var files []reporter.FileInfo
		if len(s.allFiles) > 0 {
			files = s.allFiles
			log.Printf("✅ [API] Using pre-computed allFiles list (%d files)", len(files))
		} else {
			log.Printf("⚠️  [API] Building files list from groups (pre-computed list empty)")
			fileMap := make(map[string]reporter.FileInfo)
			for _, group := range s.report.SizeGroups {
				for _, file := range group.Files {
					fileMap[file.Path] = file
				}
			}
			log.Printf("📊 [API] Collected %d files from SizeGroups", len(fileMap))

			for _, group := range s.report.SimilarGroups {
				for _, file := range group.Files {
					fileMap[file.Path] = file
				}
			}
			files = make([]reporter.FileInfo, 0, len(fileMap))
			for _, file := range fileMap {
				files = append(files, file)
			}
			log.Printf("📊 [API] Final file list: %d unique files", len(files))
		}

		log.Printf("✅ [API] Returning %d files to gallery view", len(files))
		return c.Status(200).JSON(fiber.Map{
			"files": files,
			"total": len(files),
		})
	})

	// Endpoint: /api/preview?path=...&internal_path=...
	api.Get("/preview", func(c *fiber.Ctx) error {
		path := c.Query("path")
		internalPath := c.Query("internal_path")
		previewType := c.Query("type", "preview")

		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		log.Printf("🎬 [API] Preview request - Path: %s, Type: %s, InternalPath: %s", path, previewType, internalPath)

		// Determine if it's a direct file or an archive
		isArchive := false
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".zip" || ext == ".rar" || ext == ".7z" || ext == ".tar" || ext == ".gz" {
			isArchive = true
		}

		log.Printf("📦 [API] Archive: %v, Format: %s", isArchive, ext)

		// 1. Handling when internalPath is NOT specified (Initial Gallery Load)
		if internalPath == "" {
			if !isArchive {
				// Direct file (image, video, model)
				log.Printf("📄 [API] Direct file (not archive), path: %s", path)

				// Special handling for STL files when format=png is requested
				formatParam := c.Query("format")
				if formatParam == "png" && ext == ".stl" {
					log.Printf("🎚️  [API] STL render requested for direct file")

					// Read the STL file
					data, err := os.ReadFile(path)
					if err != nil {
						log.Printf("❌ [API] Failed to read STL file: %v", err)
						return c.Status(404).SendString("File not found")
					}
					log.Printf("✅ [API] Read STL file (%.1f KB)", float64(len(data))/1024)

					// Parse and render to PNG
					info, err := stl.ParseSTL(data)
					if err != nil {
						log.Printf("❌ [API] Failed to parse STL file: %v", err)
						return c.Status(422).SendString("Invalid STL file")
					}

					pngData, err := stl.RenderToPNG(info, 800, 600)
					if err != nil {
						log.Printf("❌ [API] Failed to render STL to PNG: %v", err)
						return c.Status(500).SendString("Render failed")
					}

					log.Printf("✅ [API] STL rendered to PNG (%.1f KB)", float64(len(pngData))/1024)
					c.Set("Content-Type", "image/png")
					return c.Send(pngData)
				}

				// Regular file: Send with correct content type
				log.Printf("📄 [API] Sending direct file with type: %s", getContentType(path))
				contentType := getContentType(path)
				c.Set("Content-Type", contentType)
				return c.SendFile(path)
			}

			// Check cache first
			log.Printf("💾 [API] Archive, checking cache for preview path...")
			info, _ := os.Stat(path)
			modTime := ""
			if info != nil {
				modTime = info.ModTime().String()
			}

			var found bool
			if s.cache != nil && previewType != "model" {
				internalPath, found = s.cache.GetPreviewPath(path, modTime)
			}

			if !found {
				log.Printf("🔍 [API] Cache miss or model type, searching for preview in archive...")

				// Archive without internal path: Find the best preview filename efficiently
				var filename string
				var err error

				if previewType == "model" {
					log.Printf("🔍 [API] Searching for STL model in: %s", path)
					filename, err = archive.FindBestSTLInArchive(path)
				} else {
					log.Printf("🔍 [API] Searching for any preview in: %s", path)
					filename, err = archive.FindPreviewPathInArchive(path)
				}

				if err != nil {
					errMsg := err.Error()
					if strings.Contains(errMsg, "invalid filter") {
						log.Printf("❌ [API] RAR file uses unsupported compression filter")
						log.Printf("    Archive: %s", path)
						log.Printf("    This RAR was likely created with WinRAR 5.0+ and uses compression methods not supported by Go's rardecode library")
						log.Printf("    Workaround: Re-save the archive with an older version of WinRAR or convert to ZIP/7Z format")
						return c.Status(415).SendString("RAR file uses unsupported compression (needs WinRAR 4.x compatibility)")
					} else if strings.Contains(errMsg, "corrupted") || strings.Contains(errMsg, "broken") {
						log.Printf("❌ [API] Archive appears corrupted: %v", err)
						log.Printf("    Archive: %s", path)
						log.Printf("    Try testing the archive integrity with WinRAR or 7-Zip")
						return c.Status(422).SendString("Archive may be corrupted: " + errMsg)
					} else if strings.Contains(errMsg, "no preview found") || strings.Contains(errMsg, "no files found") {
						log.Printf("⚠️  [API] Archive found but no previewable files inside")
						log.Printf("    Archive: %s", path)
						log.Printf("    Supported preview types: *.jpg, *.png, *.webp, *.mp4, *.webm, *.stl, *.obj")
						return c.Status(204).SendString("No previewable files found in archive")
					} else {
						log.Printf("❌ [API] Preview search failed: %v", err)
						return c.Status(404).SendString(errMsg)
					}
				}

				log.Printf("✅ [API] Found preview: %s", filename)
				internalPath = filename
				foundExt := strings.ToLower(filepath.Ext(internalPath))
				isFoundSTL := foundExt == ".stl"
				log.Printf("📋 [API] Preview extension: %s (STL=%v)", foundExt, isFoundSTL)

				// Save to cache (only if standard preview)
				if s.cache != nil && previewType != "model" {
					log.Printf("💾 [API] Saving to cache...")
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
		// ALWAYS render STLs to PNG for gallery preview (regardless of format param)
		isSTRRender := strings.ToLower(filepath.Ext(internalPath)) == ".stl"
		if isSTRRender {
			fileExt = ".png"
			// Use different cache key for STL-to-PNG rendering to avoid conflicts with old STL cache
			cacheKey = cacheKey + "_stl_png_render"
			log.Printf("🎚️  [API] STL detected - will render to PNG, using separate cache key: %s", cacheKey)
		} else {
			log.Printf("📝 [API] Non-STL file - ext=%s, no rendering needed", filepath.Ext(internalPath))
		}

		// 1. Try to get thumbnail from DB cache
		if s.cache != nil {
			if cachedData, cType, ok := s.cache.GetThumbnail(cacheKey); ok {
				log.Printf("✅ [API] Thumbnail found in cache (%.1f KB, type: %s)", float64(len(cachedData))/1024, cType)

				// Security check: if it's supposed to be PNG but cache says STL, ignore it (corrupted cache)
				if isSTRRender && cType != "image/png" && cType != "image/jpeg" {
					log.Printf("⚠️  [API] Cache has wrong type for STL render (%s), re-rendering", cType)
				} else {
					c.Set("X-Internal-Path", internalPath)
					c.Set("Content-Type", cType)
					return c.Send(cachedData)
				}
			}
		}

		log.Printf("🔓 [API] Thumbnail not in cache, enqueueing background job for extraction...")

			// Try a fast synchronous extraction if we can acquire the semaphore immediately
			select {
			case s.previewSem <- struct{}{}:
				// Fast-path: perform extraction inline to improve UX for first request
				log.Printf("🔧 [API] Fast-path extraction for: %s", internalPath)
				dataFast, errFast := archive.GetFileFromArchive(path, internalPath)
				if errFast == nil {
					// SPECIAL CASE: STL render
					if isSTRRender {
						info, err := stl.ParseSTL(dataFast)
						if err == nil {
							if pngData, err := stl.RenderToPNG(info, 800, 600); err == nil {
								dataFast = pngData
								log.Printf("✅ [API] Fast-path STL rendered for: %s", internalPath)
							} else {
								log.Printf("❌ [API] Fast-path STL render failed: %v", err)
							}
						} else {
							log.Printf("❌ [API] Fast-path STL parse failed: %v", err)
						}
					}

					// Resize if image
					contentTypeFast := getContentType(cacheKey + fileExt)
					if strings.HasPrefix(contentTypeFast, "image/") {
						img, _, err := image.Decode(bytes.NewReader(dataFast))
						if err == nil {
							bounds := img.Bounds()
							if bounds.Dx() > 256 || bounds.Dy() > 256 {
								img = resize.Thumbnail(256, 256, img, resize.Bilinear)
								buf := new(bytes.Buffer)
								if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}); err == nil {
									dataFast = buf.Bytes()
									contentTypeFast = "image/jpeg"
								}
							}
						}
					}

					if s.cache != nil {
						s.cache.PutThumbnail(cacheKey, dataFast, contentTypeFast)
						go s.checkCacheLimit()
					}

					<-s.previewSem
					c.Set("X-Internal-Path", internalPath)
					c.Set("Content-Type", contentTypeFast)
					log.Printf("✅ [API] Fast-path returning preview (%.1f KB)", float64(len(dataFast))/1024)
					return c.Send(dataFast)
				}
				// Fast-path failed, release semaphore and fallthrough to enqueue
				<-s.previewSem
				log.Printf("⚠️  [API] Fast-path extraction failed for %s, enqueuing", internalPath)
			default:
				// Could not acquire semaphore immediately; continue to enqueue
			}

			// Use cacheKey as job ID (unique per file/internal path)
		jobID := cacheKey

		// If job already exists, return 202
		if v, ok := s.jobStatus.Load(jobID); ok {
			status := v.(string)
			log.Printf("🔁 [API] Job already exists: %s (status=%s)", jobID, status)
			c.Set("X-Job-ID", jobID)
			return c.Status(202).JSON(fiber.Map{"status": status, "job": jobID, "poll": "/api/preview-status?job=" + jobID})
		}

		// Enqueue job
		// compute modTime at enqueue time
		modTimeLocal := ""
		if info2, err := os.Stat(path); err == nil && info2 != nil {
			modTimeLocal = info2.ModTime().String()
		}
		pj := &previewJob{
			JobID:       jobID,
			Path:        path,
			Internal:    internalPath,
			CacheKey:    cacheKey,
			FileExt:     fileExt,
			ModTime:     modTimeLocal,
			IsSTRRender: isSTRRender,
		}
		s.jobStatus.Store(jobID, "queued")
		select {
		case s.jobQueue <- pj:
			log.Printf("📥 [API] Enqueued preview job: %s", jobID)
			c.Set("X-Job-ID", jobID)
			// Short wait: block briefly (up to 2s) to see if worker completes fast
			shortTimeout := time.After(2 * time.Second)
			shortTicker := time.NewTicker(150 * time.Millisecond)
			defer shortTicker.Stop()
			for {
				select {
				case <-shortTimeout:
					// If client explicitly asked to wait longer, honor it
					if c.Query("wait") == "1" {
						timeout := time.After(10 * time.Second)
						ticker := time.NewTicker(200 * time.Millisecond)
						defer ticker.Stop()
						for {
							select {
							case <-timeout:
								return c.Status(202).JSON(fiber.Map{"status": "processing", "job": jobID})
							case <-ticker.C:
								if v, ok := s.jobStatus.Load(jobID); ok {
									if v.(string) == "done" {
										if s.cache != nil {
											if cachedData, cType, ok := s.cache.GetThumbnail(cacheKey); ok {
												c.Set("Content-Type", cType)
												return c.Send(cachedData)
											}
										}
										return c.Status(202).JSON(fiber.Map{"status": "done", "job": jobID})
									}
									if v.(string) == "failed" {
										return c.Status(500).JSON(fiber.Map{"status": "failed", "job": jobID})
									}
								}
							}
						}
					}
					return c.Status(202).JSON(fiber.Map{"status": "queued", "job": jobID, "poll": "/api/preview-status?job=" + jobID})
				case <-shortTicker.C:
					if v, ok := s.jobStatus.Load(jobID); ok {
						if v.(string) == "done" {
							if s.cache != nil {
								if cachedData, cType, ok := s.cache.GetThumbnail(cacheKey); ok {
									c.Set("Content-Type", cType)
									return c.Send(cachedData)
								}
							}
							return c.Status(202).JSON(fiber.Map{"status": "done", "job": jobID})
						}
						if v.(string) == "failed" {
							return c.Status(500).JSON(fiber.Map{"status": "failed", "job": jobID})
						}
					}
				}
			}
			// If client asked to wait, block for up to 10s
			if c.Query("wait") == "1" {
				timeout := time.After(10 * time.Second)
				ticker := time.NewTicker(200 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-timeout:
						return c.Status(202).JSON(fiber.Map{"status": "processing", "job": jobID})
					case <-ticker.C:
						if v, ok := s.jobStatus.Load(jobID); ok {
							if v.(string) == "done" {
								if s.cache != nil {
									if cachedData, cType, ok := s.cache.GetThumbnail(cacheKey); ok {
										c.Set("Content-Type", cType)
										return c.Send(cachedData)
									}
								}
								return c.Status(202).JSON(fiber.Map{"status": "done", "job": jobID})
							}
							if v.(string) == "failed" {
								return c.Status(500).JSON(fiber.Map{"status": "failed", "job": jobID})
							}
						}
					}
				}
			}
			return c.Status(202).JSON(fiber.Map{"status": "queued", "job": jobID, "poll": "/api/preview-status?job=" + jobID})
		default:
			// Queue is full
			log.Printf("⚠️  [API] Job queue full, rejecting: %s", jobID)
			return c.Status(503).SendString("Server busy, try again")
		}
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

	// Preview job status endpoint
	api.Get("/preview-status", func(c *fiber.Ctx) error {
		jobID := c.Query("job")
		if jobID == "" {
			return c.Status(400).SendString("job is required")
		}
		if v, ok := s.jobStatus.Load(jobID); ok {
			status := v.(string)
			// If done, try returning cached thumbnail directly
			if status == "done" && s.cache != nil {
				if data, cType, ok := s.cache.GetThumbnail(jobID); ok {
					c.Set("Content-Type", cType)
					return c.Send(data)
				}
			}
			return c.Status(200).JSON(fiber.Map{"job": jobID, "status": status})
		}
		return c.Status(404).JSON(fiber.Map{"error": "job not found"})
	})

	api.Get("/open", func(c *fiber.Ctx) error {
		path := c.Query("path")
		mode := c.Query("mode", "reveal") // "reveal" or "launch"
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		// Fix Windows drive letter paths that lost the backslash during URL encoding
		// e.g., "R:Pr\..." becomes "R:\Pr\..."
		if len(path) > 2 && path[1] == ':' && path[2] != '\\' && path[2] != '/' {
			path = path[:2] + "\\" + path[2:]
			log.Printf("🔧 [API] Fixed drive letter path to: %s", path)
		}

		log.Printf("📂 [API] Open request: path=%s, mode=%s", path, mode)

		// For reveal mode, always open the parent directory
		// This avoids issues with non-existent files or special characters in paths
		if mode == "reveal" {
			// Just open the directory that contains the file
			openPath := filepath.Dir(path)
			log.Printf("📂 [API] Opening directory: %s", openPath)

			var cmd *exec.Cmd
			switch runtime.GOOS {
			case "windows":
				cmd = exec.Command("explorer.exe", openPath)
			case "darwin":
				cmd = exec.Command("open", openPath)
			case "linux":
				cmd = exec.Command("xdg-open", openPath)
			default:
				return c.Status(500).SendString("Unsupported OS")
			}

			if err := cmd.Start(); err != nil {
				log.Printf("❌ [API] Failed to open directory: %v", err)
				return c.Status(500).SendString(err.Error())
			}
			log.Printf("✅ [API] Successfully opened directory: %s", openPath)
			return c.SendStatus(200)
		}

		// For launch mode, try to open the file if it exists
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path)
		case "darwin":
			cmd = exec.Command("open", path)
		case "linux":
			cmd = exec.Command("xdg-open", path)
		default:
			return c.Status(500).SendString("Unsupported OS")
		}

		if err := cmd.Start(); err != nil {
			log.Printf("❌ [API] Failed to launch file: %v", err)
			return c.Status(500).SendString(err.Error())
		}
		log.Printf("✅ [API] Successfully launched file: %s", path)
		return c.SendStatus(200)
	})

	api.Post("/recompress", func(c *fiber.Ctx) error {
		type recompressRequest struct {
			Path string `json:"path"`
		}
		var req recompressRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Invalid request body")
		}
		if req.Path == "" {
			return c.Status(400).SendString("Path is required")
		}

		if len(req.Path) > 2 && req.Path[1] == ':' && req.Path[2] != '\\' && req.Path[2] != '/' {
			req.Path = req.Path[:2] + "\\" + req.Path[2:]
		}

		if _, err := os.Stat(req.Path); os.IsNotExist(err) {
			return c.Status(404).SendString("File not found")
		}

		jobID := s.createRecompressJob(req.Path)
		go func(jobID, path string) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("❌ [API] Panic during recompress for %s: %v", path, r)
					s.completeRecompressJob(jobID, 0, "Failed", fmt.Sprintf("panic: %v", r), 0)
				}
			}()

			s.updateRecompressJob(jobID, "running", 10, "Preparing recompression")
			input := path
			output := path + ".recompressed.zip"
			var err error
			if filepath.Ext(input) == ".zip" || filepath.Ext(input) == ".rar" || filepath.Ext(input) == ".7z" {
				s.updateRecompressJob(jobID, "running", 25, "Unpacking archive contents")
				_, err = archive.RecompressArchive(input, output)
			} else {
				s.updateRecompressJob(jobID, "running", 25, "Wrapping file into a fresh archive")
				_, err = archive.RecompressFile(input, output)
			}
			if err != nil {
				s.completeRecompressJob(jobID, 0, "failed", err.Error(), 0)
				return
			}

			s.updateRecompressJob(jobID, "running", 80, "Replacing original file")
			if err := os.Remove(input); err != nil {
				s.completeRecompressJob(jobID, 0, "failed", err.Error(), 0)
				return
			}
			if err := os.Rename(output, input); err != nil {
				s.completeRecompressJob(jobID, 0, "failed", err.Error(), 0)
				return
			}

			sizeAfter := int64(0)
			if info, statErr := os.Stat(input); statErr == nil {
				sizeAfter = info.Size()
			}
			s.completeRecompressJob(jobID, 100, "completed", "", sizeAfter)
		}(jobID, req.Path)

		return c.JSON(fiber.Map{"job_id": jobID, "status": "queued"})
	})

	api.Get("/recompress/:jobID", func(c *fiber.Ctx) error {
		job := s.getRecompressJob(c.Params("jobID"))
		if job == nil {
			return c.Status(404).SendString("Job not found")
		}
		return c.JSON(job)
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

	// Start background preview workers
	go func() {
		workerCount := 6
		for i := 0; i < workerCount; i++ {
			go func(idx int) {
				log.Printf("🔧 [PREVIEW WORKER] started %d", idx)
				for job := range s.jobQueue {
					log.Printf("🔨 [PREVIEW WORKER] processing job %s", job.JobID)
					s.jobStatus.Store(job.JobID, "processing")
					// Acquire extraction semaphore
					s.previewSem <- struct{}{}
					data, err := archive.GetFileFromArchive(job.Path, job.Internal)
					if err != nil {
						log.Printf("❌ [PREVIEW WORKER] extraction failed for %s: %v", job.JobID, err)
						s.jobStatus.Store(job.JobID, "failed")
						<-s.previewSem
						continue
					}
					// If STL render requested
					if job.IsSTRRender {
						info, err := stl.ParseSTL(data)
						if err == nil {
							pngData, err := stl.RenderToPNG(info, 800, 600)
							if err == nil {
								data = pngData
								log.Printf("✅ [PREVIEW WORKER] STL rendered for %s", job.JobID)
							} else {
								log.Printf("❌ [PREVIEW WORKER] STL render failed for %s: %v", job.JobID, err)
								s.jobStatus.Store(job.JobID, "failed")
								<-s.previewSem
								continue
							}
						} else {
							log.Printf("❌ [PREVIEW WORKER] STL parse failed for %s", job.JobID)
							s.jobStatus.Store(job.JobID, "failed")
							<-s.previewSem
							continue
						}
					}
					// Determine content type and resize if image
					contentType := getContentType(job.CacheKey + job.FileExt)
					if strings.HasPrefix(contentType, "image/") {
						img, _, err := image.Decode(bytes.NewReader(data))
						if err == nil {
							bounds := img.Bounds()
							if bounds.Dx() > 256 || bounds.Dy() > 256 {
								img = resize.Thumbnail(256, 256, img, resize.Bilinear)
								buf := new(bytes.Buffer)
								if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}); err == nil {
									data = buf.Bytes()
									contentType = "image/jpeg"
								}
							}
						}
					}
					// Save to cache
					if s.cache != nil {
						s.cache.PutThumbnail(job.CacheKey, data, contentType)
						go s.checkCacheLimit()
					}
					s.jobStatus.Store(job.JobID, "done")
					<-s.previewSem
					log.Printf("✅ [PREVIEW WORKER] job %s done", job.JobID)
				}
			}(i)
		}
	}()
	return app.Listen(s.addr)
}

func (s *Server) performFullScan(cfg *config.AppConfig) {
	var scanLogMsg string
	if cfg.ScanFullSystem {
		scanLogMsg = "🔍 Starting web-triggered full system scan"
	} else {
		scanLogMsg = fmt.Sprintf("🔍 Starting web-triggered scan: %s", cfg.Directory)
	}
	log.Printf("%s", scanLogMsg)

	s.mu.Lock()
	s.report = &reporter.Report{
		Status: "analyzing",
	}
	s.allFiles = []reporter.FileInfo{}
	s.mu.Unlock()

	startTime := time.Now()

	var files []scanner.ArchiveFile
	var err error

	if cfg.ScanFullSystem {
		// Scan multiple root paths
		rootPaths := scanner.GetRootPaths()
		log.Printf("🔍 Detected root paths: %v", rootPaths)
		files, err = scanner.ScanMultiplePaths(rootPaths, cfg.Recursive)
	} else {
		// Scan single directory
		files, err = scanner.ScanDirectory(cfg.Directory, cfg.Recursive)
	}

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
	scanFullSystem := false
	if s.config != nil {
		threshold = s.config.Threshold
		scanFullSystem = s.config.ScanFullSystem
	}
	s.mu.Unlock()

	log.Printf("📝 Web-triggered Step 3 analysis started...")
	startTime := time.Now()

	// Need scanner.ArchiveFile objects.
	var files []scanner.ArchiveFile
	if scanFullSystem {
		rootPaths := scanner.GetRootPaths()
		files, _ = scanner.ScanMultiplePaths(rootPaths, true)
	} else {
		files, _ = scanner.ScanDirectory(scanDir, true)
	}

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
	scanFullSystem := false
	if s.config != nil {
		threshold = s.config.Threshold
		scanFullSystem = s.config.ScanFullSystem
	}
	s.mu.Unlock()

	log.Printf("🎨 Web-triggered Visual analysis started...")

	var files []scanner.ArchiveFile
	if scanFullSystem {
		rootPaths := scanner.GetRootPaths()
		files, _ = scanner.ScanMultiplePaths(rootPaths, true)
	} else {
		files, _ = scanner.ScanDirectory(scanDir, true)
	}

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
					Name:        f.Name,
					Path:        f.Path,
					Size:        f.Size,
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
