package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Directory  string `json:"directory"`
	TrashPath  string `json:"trash_path"`
	Threshold  int    `json:"threshold"`
	Recursive  bool   `json:"recursive"`
	LeaveRef   bool   `json:"leave_ref"`
	DeleteMode string `json:"delete_mode"`
	Port       int    `json:"port"`
}

func GetConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "archive-finder-settings.json"
	}
	return filepath.Join(filepath.Dir(exePath), "archive-finder-settings.json")
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
