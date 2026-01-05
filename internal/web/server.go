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
	addr      string
	report    *reporter.Report
	trashPath string
	leaveRef  bool
	debug     bool
	mu        sync.Mutex
}

// NewServer creates a new web dashboard server
func NewServer(port int, report *reporter.Report, trashPath string, leaveRef bool) *Server {
	return &Server{
		addr:      fmt.Sprintf(":%d", port),
		report:    report,
		trashPath: trashPath,
		leaveRef:  leaveRef,
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

	api.Get("/report", func(c *fiber.Ctx) error {
		if c.Query("exclude_similar") == "true" {
			// Create a copy without similar pairs
			reportCopy := *s.report
			reportCopy.SimilarPairs = nil
			return c.Status(200).JSON(reportCopy)
		}
		return c.Status(200).JSON(s.report)
	})

	api.Get("/stats", func(c *fiber.Ctx) error {
		return c.Status(200).JSON(fiber.Map{
			"totalFiles": s.report.TotalFiles,
			"duplicates": len(s.report.SizeGroups),
			"similar":    len(s.report.SimilarPairs),
			"duration":   s.report.AnalysisDuration,
		})
	})

	api.Get("/preview", func(c *fiber.Ctx) error {
		path := c.Query("path")
		if path == "" {
			return c.Status(400).SendString("Path is required")
		}

		data, filename, err := archive.FindFirstImageInArchive(path)
		if err != nil {
			return c.Status(404).SendString(err.Error())
		}

		// Set content type based on extension
		ext := strings.ToLower(filepath.Ext(filename))
		contentType := "image/jpeg"
		switch ext {
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		}

		c.Set("Content-Type", contentType)
		return c.Send(data)
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

		// Remove from Similarity Pairs
		newPairs := make([]reporter.SimilarPair, 0)
		for _, p := range s.report.SimilarPairs {
			if p.File1.Path != req.Path && p.File2.Path != req.Path {
				newPairs = append(newPairs, p)
			}
		}
		s.report.SimilarPairs = newPairs
		s.report.SimilarCount = len(newPairs)

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

	// Placeholder for static dashboard (will be overwritten by app.Static but good for fallback)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("Archive Duplicate Finder Dashboard API is running")
	})

	log.Printf("🚀 Web Dashboard available at: http://localhost%s", s.addr)
	return app.Listen(s.addr)
}
