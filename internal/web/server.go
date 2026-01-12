package web

import (
	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/reporter"
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
	addr         string
	report       *reporter.Report
	trashPath    string
	leaveRef     bool
	debug        bool
	runStep3Func func()
	allFiles     []reporter.FileInfo
	mu           sync.Mutex
}

// NewServer creates a new web dashboard server
func NewServer(port int, report *reporter.Report, trashPath string, leaveRef bool, runStep3Func func(), allFiles []reporter.FileInfo) *Server {
	return &Server{
		addr:         fmt.Sprintf(":%d", port),
		report:       report,
		trashPath:    trashPath,
		leaveRef:     leaveRef,
		runStep3Func: runStep3Func,
		allFiles:     allFiles,
	}
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
		if s.runStep3Func != nil {
			go s.runStep3Func()      // Run in background
			return c.SendStatus(202) // Accepted
		}
		return c.Status(501).SendString("Step 3 runner not configured")
	})

	api.Get("/report", func(c *fiber.Ctx) error {
		if c.Query("exclude_similar") == "true" {
			// Create a copy without similar groups
			reportCopy := *s.report
			reportCopy.SimilarGroups = nil
			return c.Status(200).JSON(reportCopy)
		}
		return c.Status(200).JSON(s.report)
	})

	api.Get("/stats", func(c *fiber.Ctx) error {
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

	api.Get("/preview", func(c *fiber.Ctx) error {
		path := c.Query("path")
		internalPath := c.Query("internal_path")
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		// Check if it's a direct STL/OBJ file
		ext := strings.ToLower(filepath.Ext(path))
		if internalPath == "" && (ext == ".stl" || ext == ".obj") {
			// Serve the file directly
			data, err := os.ReadFile(path)
			if err != nil {
				return c.Status(404).SendString(err.Error())
			}
			contentType := "model/stl"
			if ext == ".obj" {
				contentType = "model/obj"
			}
			c.Set("Content-Type", contentType)
			return c.Send(data)
		}

		var data []byte
		var filename string
		var err error

		if internalPath != "" {
			// Fetch specific file from archive
			data, err = archive.GetFileFromArchive(path, internalPath)
			if err != nil {
				return c.Status(404).SendString(err.Error())
			}
			filename = internalPath
		} else {
			// Otherwise, try to extract default (largest) preview from archive
			data, filename, err = archive.FindPreviewInArchive(path)
			if err != nil {
				return c.Status(404).SendString(err.Error())
			}
		}

		// Set content type based on extension
		fileExt := strings.ToLower(filepath.Ext(filename))
		contentType := "image/jpeg"
		switch fileExt {
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		case ".stl":
			contentType = "model/stl"
		case ".obj":
			contentType = "model/obj"
		}

		c.Set("Content-Type", contentType)
		c.Set("X-Internal-Path", filename) // Let the client know which file was selected
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
