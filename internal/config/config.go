package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Directory    string  `json:"directory"`
	TrashPath    string  `json:"trash_path"`
	Threshold    int     `json:"threshold"`
	Recursive    bool    `json:"recursive"`
	LeaveRef     bool    `json:"leave_ref"`
	DeleteMode   string  `json:"delete_mode"`
	Port         int     `json:"port"`
	CacheLimitGB float64 `json:"cache_limit_gb"`
}

func GetConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "archive-finder-settings.json"
	}
	return filepath.Join(filepath.Dir(exePath), "archive-finder-settings.json")
}

func GetBaseCacheDir() string {
	exePath, err := os.Executable()
	var baseDir string
	if err != nil {
		baseDir = "."
	} else {
		baseDir = filepath.Dir(exePath)
	}
	baseCacheDir := filepath.Join(baseDir, ".cache")
	os.MkdirAll(baseCacheDir, 0755)
	return baseCacheDir
}

func GetPreviewCacheDir() string {
	dir := filepath.Join(GetBaseCacheDir(), "previews")
	os.MkdirAll(dir, 0755)
	return dir
}

func GetLogoCacheDir() string {
	dir := filepath.Join(GetBaseCacheDir(), "logos")
	os.MkdirAll(dir, 0755)
	return dir
}

func GetDatabasePath() string {
	return filepath.Join(GetBaseCacheDir(), "archive-finder-cache.db")
}

func GetCacheDir() string {
	return GetPreviewCacheDir()
}

func LoadConfig() (*AppConfig, error) {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &AppConfig{
			Threshold: 70,
			Recursive: true,
			Port:      8080,
		}, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *AppConfig) error {
	path := GetConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func GetCacheSize() (int64, error) {
	var size int64
	cacheDir := GetBaseCacheDir()
	err := filepath.Walk(cacheDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	
	// Add DB size if not explicitly in cache dir (it is in GetBaseCacheDir, so the walk should cover it)
	// Just to be safe, if we move the db this ensures it's included.
	dbPath := GetDatabasePath()
	if dbInfo, err := os.Stat(dbPath); err == nil {
		// If DB is outside cacheDir, add it. If inside, Walk already counted it.
		if filepath.Dir(dbPath) != cacheDir {
			size += dbInfo.Size()
		}
	}
	
	return size, err
}

func ClearCache() error {
	cacheDir := GetBaseCacheDir()
	// Define subdirs we want to clear but keep base
	subdirs := []string{"previews", "logos"}
	for _, sub := range subdirs {
		dir := filepath.Join(cacheDir, sub)
		os.RemoveAll(dir)
		os.MkdirAll(dir, 0755)
	}
	return nil
}
