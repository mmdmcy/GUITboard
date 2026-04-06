package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	appDirName = "GUITboard"
	fileName   = "config.json"
)

type Config struct {
	RootPath           string    `json:"root_path"`
	AutoRefreshSeconds int       `json:"auto_refresh_seconds"`
	LastOpened         time.Time `json:"last_opened"`
	LastScan           time.Time `json:"last_scan"`
	WindowWidth        float32   `json:"window_width"`
	WindowHeight       float32   `json:"window_height"`
}

func Default() Config {
	return Config{
		AutoRefreshSeconds: 30,
		WindowWidth:        1480,
		WindowHeight:       920,
	}
}

func Load() (Config, error) {
	cfg := Default()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}

	if cfg.AutoRefreshSeconds <= 0 {
		cfg.AutoRefreshSeconds = 30
	}
	if cfg.WindowWidth < 960 {
		cfg.WindowWidth = 1480
	}
	if cfg.WindowHeight < 700 {
		cfg.WindowHeight = 920
	}

	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, appDirName, fileName), nil
}
