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
	"time"

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

	// Configure SQLite for concurrent access
	// See: https://www.sqlite.org/pragma.html
	pragmas := []string{
		// Enable WAL mode for better concurrency
		"PRAGMA journal_mode=WAL",
		// Set timeout to 30 seconds for busy waits
		"PRAGMA busy_timeout=30000",
		// Enable foreign keys
		"PRAGMA foreign_keys=ON",
		// Synchronous mode: NORMAL (faster than FULL, safer than OFF)
		"PRAGMA synchronous=NORMAL",
		// Increase cache size for better performance
		"PRAGMA cache_size=10000",
		// Enable memory-mapped I/O if available
		"PRAGMA mmap_size=30000000",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Printf("⚠️  Database: Could not set pragma '%s': %v", pragma, err)
		}
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)   // Allow multiple concurrent connections
	db.SetMaxIdleConns(5)    // Keep some idle for reuse
	db.SetConnMaxLifetime(0) // Unlimited connection lifetime

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	log.Printf("✅ Database: Connected and configured for concurrent access")

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

// executeWithRetry executes a database operation with exponential backoff retry logic
func (c *Cache) executeWithRetry(fn func(*sql.DB) error, operationName string) error {
	maxRetries := 5
	backoff := time.Millisecond * 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn(c.db)
		if err == nil {
			if attempt > 0 {
				log.Printf("✅ [CACHE] %s succeeded on attempt %d", operationName, attempt+1)
			}
			return nil
		}

		// Check if it's a "database is locked" error
		errStr := err.Error()
		if !(errStr == "database is locked (5) (SQLITE_BUSY)" || 
			 errStr == "database is locked" ||
			 errStr == "database table is locked") {
			// Not a lock error, return immediately
			return err
		}

		if attempt < maxRetries-1 {
			log.Printf("⏳ [CACHE] %s locked, retrying in %v (attempt %d/%d)...", 
				operationName, backoff, attempt+1, maxRetries)
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("%s failed after %d retries: database is locked", operationName, maxRetries)
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
	_ = c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("INSERT OR REPLACE INTO scan_cache (fingerprint, results_json) VALUES (?, ?)", fingerprint, string(data))
		return err
	}, "PutSimilarities")
}

func (c *Cache) GetPreviewPath(path string, modTime string) (string, bool) {
	var internalPath string
	var cachedModTime string
	err := c.db.QueryRow("SELECT internal_path, mod_time FROM preview_cache WHERE path = ?", path).Scan(&internalPath, &cachedModTime)
	
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("💾 [CACHE] Preview NOT found for: %s (cache miss)", path)
		} else {
			log.Printf("⚠️  [CACHE] Error querying preview cache: %v", err)
		}
		return "", false
	}
	
	if cachedModTime != modTime {
		log.Printf("🔄 [CACHE] Preview found but OUTDATED for: %s (was updated: %s → %s)", path, cachedModTime, modTime)
		return "", false
	}
	
	log.Printf("✅ [CACHE] Preview cache HIT: %s → %s", path, internalPath)
	return internalPath, true
}

func (c *Cache) PutPreviewPath(path string, internalPath string, modTime string) {
	err := c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("INSERT OR REPLACE INTO preview_cache (path, internal_path, mod_time) VALUES (?, ?, ?)", path, internalPath, modTime)
		return err
	}, "PutPreviewPath")
	if err != nil {
		log.Printf("❌ [CACHE] Error saving preview to cache: %v", err)
	} else {
		log.Printf("💾 [CACHE] Preview saved to cache: %s → %s", path, internalPath)
	}
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
	_ = c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("INSERT OR REPLACE INTO visual_cache (path, phash, mod_time) VALUES (?, ?, ?)", path, int64(phash), modTime)
		return err
	}, "PutVisualHash")
}

func (c *Cache) AddIgnoredGroup(hash string) {
	_ = c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("INSERT OR REPLACE INTO ignored_groups (hash) VALUES (?)", hash)
		return err
	}, "AddIgnoredGroup")
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
		if err == sql.ErrNoRows {
			log.Printf("🖼️  [CACHE] Thumbnail cache MISS for: %s", cacheKey)
		} else {
			log.Printf("⚠️  [CACHE] Error querying thumbnail cache: %v", err)
		}
		return nil, "", false
	}
	// Update last accessed
	_, _ = c.db.Exec("UPDATE thumbnail_cache SET last_accessed = CURRENT_TIMESTAMP WHERE cache_key = ?", cacheKey)
	log.Printf("✅ [CACHE] Thumbnail cache HIT for: %s (%.1f KB, type: %s)", cacheKey, float64(len(data))/1024, contentType)
	return data, contentType, true
}

func (c *Cache) PutThumbnail(cacheKey string, data []byte, contentType string) {
	size := len(data)
	err := c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("INSERT OR REPLACE INTO thumbnail_cache (cache_key, image_data, content_type, size, last_accessed) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)", cacheKey, data, contentType, size)
		return err
	}, "PutThumbnail")
	if err != nil {
		log.Printf("❌ [CACHE] Error saving thumbnail to cache: %v", err)
	} else {
		log.Printf("💾 [CACHE] Thumbnail saved to cache: %s (%.1f KB, type: %s)", cacheKey, float64(size)/1024, contentType)
	}
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
	return c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM thumbnail_cache WHERE cache_key IN (
			SELECT cache_key FROM thumbnail_cache ORDER BY last_accessed ASC LIMIT (SELECT COUNT(*)/2 FROM thumbnail_cache)
		)`)
		if err != nil {
			return err
		}
		_, _ = db.Exec("VACUUM")
		return nil
	}, "CleanThumbnailsIfNeeded")
}

func (c *Cache) ClearAllThumbnails() error {
	log.Printf("🗑️  [CACHE] Clearing ALL thumbnails from cache...")
	
	countErr := c.db.QueryRow("SELECT COUNT(*) FROM thumbnail_cache").Scan(new(int))
	if countErr != nil {
		log.Printf("⚠️  [CACHE] Error counting thumbnails before clear: %v", countErr)
	}
	
	err := c.executeWithRetry(func(db *sql.DB) error {
		_, err := db.Exec("DELETE FROM thumbnail_cache")
		if err != nil {
			return err
		}
		_, _ = db.Exec("VACUUM")
		return nil
	}, "ClearAllThumbnails")
	
	if err != nil {
		log.Printf("❌ [CACHE] Error clearing thumbnails: %v", err)
		return err
	}
	
	log.Printf("✅ [CACHE] All thumbnails cleared successfully")
	return nil
}
