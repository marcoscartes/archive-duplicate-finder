package db

import (
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

type Cache struct {
	db *sql.DB
}

func NewCache() (*Cache, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	dbPath := filepath.Join(configDir, "archive-finder-cache.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS file_metadata (
			path TEXT PRIMARY KEY,
			size INTEGER,
			mod_time TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS scan_cache (
			fingerprint TEXT PRIMARY KEY,
			results_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS preview_cache (
			path TEXT PRIMARY KEY,
			internal_path TEXT,
			mod_time TEXT
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return &Cache{db: db}, nil
}

func (c *Cache) Close() error {
	return c.db.Close()
}

func (c *Cache) CalculateFingerprint(files []scanner.ArchiveFile) string {
	// Sort files by path to ensure consistent hash
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.Path))
		h.Write([]byte(f.ModTime.String()))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (c *Cache) GetSimilarities(fingerprint string) ([]reporter.SimilarityGroup, bool) {
	var jsonStr string
	err := c.db.QueryRow("SELECT results_json FROM scan_cache WHERE fingerprint = ?", fingerprint).Scan(&jsonStr)
	if err != nil {
		return nil, false
	}

	var groups []reporter.SimilarityGroup
	if err := json.Unmarshal([]byte(jsonStr), &groups); err != nil {
		return nil, false
	}
	return groups, true
}

func (c *Cache) PutSimilarities(fingerprint string, groups []reporter.SimilarityGroup) {
	data, err := json.Marshal(groups)
	if err != nil {
		return
	}
	_, _ = c.db.Exec("INSERT OR REPLACE INTO scan_cache (fingerprint, results_json) VALUES (?, ?)", fingerprint, string(data))
}

func (c *Cache) GetPreviewPath(path string, modTime string) (string, bool) {
	var internalPath string
	var cachedModTime string
	err := c.db.QueryRow("SELECT internal_path, mod_time FROM preview_cache WHERE path = ?", path).Scan(&internalPath, &cachedModTime)
	if err != nil || cachedModTime != modTime {
		return "", false
	}
	return internalPath, true
}

func (c *Cache) PutPreviewPath(path string, internalPath string, modTime string) {
	_, _ = c.db.Exec("INSERT OR REPLACE INTO preview_cache (path, internal_path, mod_time) VALUES (?, ?, ?)", path, internalPath, modTime)
}
