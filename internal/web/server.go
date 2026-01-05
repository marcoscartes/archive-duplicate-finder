package web

import (
	"archive-duplicate-finder/internal/archive"
	"archive-duplicate-finder/internal/reporter"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// Server represents the web dashboard server
type Server struct {
	addr   string
	report *reporter.Report
}

// NewServer creates a new web dashboard server
func NewServer(port int, report *reporter.Report) *Server {
	return &Server{
		addr:   fmt.Sprintf(":%d", port),
		report: report,
	}
}

// Start starts the web server
func (s *Server) Start() error {
	app := fiber.New(fiber.Config{
		AppName: "Archive Duplicate Finder Dashboard",
	})

	// Enable CORS
	app.Use(cors.New())

	// API Routes
	api := app.Group("/api")

	api.Get("/report", func(c *fiber.Ctx) error {
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

	// Serve static dashboard files
	app.Static("/", "./ui/out")

	// Placeholder for static dashboard (will be overwritten by app.Static but good for fallback)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(200).SendString("Archive Duplicate Finder Dashboard API is running")
	})

	fmt.Printf("\n🚀 Web Dashboard available at: http://localhost%s\n", s.addr)
	return app.Listen(s.addr)
}
