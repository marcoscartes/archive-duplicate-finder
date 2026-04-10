package db

import (
	"archive-duplicate-finder/internal/config"
	"archive-duplicate-finder/internal/reporter"
	"archive-duplicate-finder/internal/scanner"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	_ "modernc.org/sqlite"
)

type Cache struct {
	db *sql.DB
}

func NewCache() (*Cache, error) {
	dbPath := config.GetDatabasePath()
	log.Printf("🗄️  Database: Loading from %s", dbPath)

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
		`CREATE TABLE IF NOT EXISTS visual_cache (
			path TEXT PRIMARY KEY,
			phash INTEGER,
			mod_time TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS ignored_groups (
			hash TEXT PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS thumbnail_cache (
			cache_key TEXT PRIMARY KEY,
			image_data BLOB,
			content_type TEXT,
			size INTEGER,
			last_accessed TEXT
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

func (c *Cache) GetVisualHash(path string, modTime string) (uint64, bool) {
	var phash int64
	var cachedModTime string
	err := c.db.QueryRow("SELECT phash, mod_time FROM visual_cache WHERE path = ?", path).Scan(&phash, &cachedModTime)
	if err != nil || cachedModTime != modTime {
		return 0, false
	}
	return uint64(phash), true
}

func (c *Cache) PutVisualHash(path string, phash uint64, modTime string) {
	_, _ = c.db.Exec("INSERT OR REPLACE INTO visual_cache (path, phash, mod_time) VALUES (?, ?, ?)", path, int64(phash), modTime)
}

func (c *Cache) AddIgnoredGroup(hash string) {
	_, _ = c.db.Exec("INSERT OR REPLACE INTO ignored_groups (hash) VALUES (?)", hash)
}

func (c *Cache) IsGroupIgnored(hash string) bool {
	var exists int
	err := c.db.QueryRow("SELECT 1 FROM ignored_groups WHERE hash = ?", hash).Scan(&exists)
	return err == nil
}

func (c *Cache) GetThumbnail(cacheKey string) ([]byte, string, bool) {
	var data []byte
	var contentType string
	err := c.db.QueryRow("SELECT image_data, content_type FROM thumbnail_cache WHERE cache_key = ?", cacheKey).Scan(&data, &contentType)
	if err != nil {
		return nil, "", false
	}
	// Update last accessed
	_, _ = c.db.Exec("UPDATE thumbnail_cache SET last_accessed = CURRENT_TIMESTAMP WHERE cache_key = ?", cacheKey)
	return data, contentType, true
}

func (c *Cache) PutThumbnail(cacheKey string, data []byte, contentType string) {
	size := len(data)
	_, _ = c.db.Exec("INSERT OR REPLACE INTO thumbnail_cache (cache_key, image_data, content_type, size, last_accessed) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)", cacheKey, data, contentType, size)
}

func (c *Cache) CleanThumbnailsIfNeeded(limitBytes int64) error {
	if limitBytes <= 0 {
		return nil
	}
	var totalSize int64
	err := c.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM thumbnail_cache").Scan(&totalSize)
	if err != nil {
		return err
	}

	if totalSize <= limitBytes {
		return nil
	}

	log.Printf("🧹 DB Thumbnail cache limit exceeded (Current: %d, Limit: %d). Cleaning up...", totalSize, limitBytes)
	_, err = c.db.Exec(`DELETE FROM thumbnail_cache WHERE cache_key IN (
		SELECT cache_key FROM thumbnail_cache ORDER BY last_accessed ASC LIMIT (SELECT COUNT(*)/2 FROM thumbnail_cache)
	)`)
	if err != nil {
		return err
	}

	_, _ = c.db.Exec("VACUUM")
	return nil
}

func (c *Cache) ClearAllThumbnails() error {
	_, err := c.db.Exec("DELETE FROM thumbnail_cache")
	if err == nil {
		_, _ = c.db.Exec("VACUUM")
	}
	return err
}
