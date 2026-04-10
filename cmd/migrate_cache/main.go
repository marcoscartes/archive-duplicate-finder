package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	configDir, _ := os.UserConfigDir()
	dbPath := filepath.Join(configDir, "archive-finder-cache.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 1. Move files
	baseDir := "c:\\Users\\gigas\\Documents\\Repos\\archive-duplicate-finder"
	oldDir := filepath.Join(baseDir, ".cache", "previews")
	newDir := filepath.Join(baseDir, ".cache", "logos")
	os.MkdirAll(newDir, 0755)

	files, err := os.ReadDir(oldDir)
	if err != nil {
		log.Printf("Could not read old dir: %v", err)
	} else {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "logo_") {
				oldPath := filepath.Join(oldDir, f.Name())
				newPath := filepath.Join(newDir, f.Name())
				os.Rename(oldPath, newPath)
				fmt.Printf("Moved %s -> %s\n", f.Name(), newDir)
			}
		}
	}

	// 2. Update DB
	// Using backward slashes for Windows replacement
	res, err := db.Exec("UPDATE creators SET logo_path = REPLACE(logo_path, '.cache\\previews', '.cache\\logos')")
	if err != nil {
		log.Printf("DB Update failed: %v", err)
	} else {
		count, _ := res.RowsAffected()
		fmt.Printf("Database paths updated: %d rows affected.\n", count)
	}
}
