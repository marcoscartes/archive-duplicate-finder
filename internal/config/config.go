package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = "archive-finder-settings.json"

// isGoBuildTemp reports whether dir is the throwaway directory that `go run`
// compiles into. Config written there is wiped when the process exits, so we
// must never treat it as a persistent location.
func isGoBuildTemp(dir string) bool {
	return strings.Contains(filepath.ToSlash(dir), "/go-build")
}

// isWritableDir checks that we can actually create a file in dir. This catches
// read-only install locations (e.g. Program Files) before we commit to them.
func isWritableDir(dir string) bool {
	testFile := filepath.Join(dir, ".aff-write-test")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

type AppConfig struct {
	Directory     string  `json:"directory"`
	TrashPath     string  `json:"trash_path"`
	Threshold     int     `json:"threshold"`
	Recursive     bool    `json:"recursive"`
	LeaveRef      bool    `json:"leave_ref"`
	DeleteMode    string  `json:"delete_mode"`
	Port          int     `json:"port"`
	CacheLimitGB  float64 `json:"cache_limit_gb"`
	ScanFullSystem bool   `json:"scan_full_system"`
}

func GetConfigPath() string {
	// Preferred location: next to the executable, so the app stays portable
	// (e.g. running from a USB drive keeps its settings with it).
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if !isGoBuildTemp(exeDir) && isWritableDir(exeDir) {
			return filepath.Join(exeDir, configFileName)
		}
	}

	// Fallback: a stable per-user config directory. Used when running via
	// `go run` (temp dir) or when the executable lives in a read-only folder.
	if cfgDir, err := os.UserConfigDir(); err == nil {
		dir := filepath.Join(cfgDir, "archive-finder")
		if os.MkdirAll(dir, 0755) == nil {
			return filepath.Join(dir, configFileName)
		}
	}

	// Last resort: the current working directory.
	return configFileName
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
