package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type appConfig struct {
	Model      *string `json:"model,omitempty"`
	BaseURL    *string `json:"base_url,omitempty"`
	OssURL     *string `json:"oss_url,omitempty"`
	APIKey     *string `json:"api_key,omitempty"`
	Username   *string `json:"username,omitempty"`
	Password   *string `json:"password,omitempty"`
	System     *string `json:"system,omitempty"`
	Stream     *bool   `json:"stream,omitempty"`
	FileFormat *string `json:"file_format,omitempty"`
	SaveImages *string `json:"save_images,omitempty"`
}

func loadAppConfig() (appConfig, error) {
	path, err := appConfigPath()
	if err != nil {
		return appConfig{}, err
	}
	sourcePath := path
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := saveAppConfig(appConfig{}); err != nil {
				return appConfig{}, err
			}
			return appConfig{}, nil
		} else {
			return appConfig{}, err
		}
	}
	if strings.TrimSpace(string(data)) == "" {
		return appConfig{}, nil
	}

	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{}, fmt.Errorf("read config %s: %w", sourcePath, err)
	}
	return cfg, nil
}

func saveAppConfig(cfg appConfig) error {
	path, err := appConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func appConfigPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("KUAIMA_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kuaima", "config.json"), nil
}

func configString(value *string, fallback string) string {
	if value != nil {
		return *value
	}
	return fallback
}

func configNonEmptyString(value *string, fallback string) string {
	if value != nil && strings.TrimSpace(*value) != "" {
		return *value
	}
	return fallback
}

func configBool(value *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	return fallback
}

func persistConfigFlags(fs *flag.FlagSet, cfg appConfig) error {
	changed := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "model":
			cfg.Model = stringPtr(f.Value.String())
			changed = true
		case "base-url":
			cfg.BaseURL = stringPtr(f.Value.String())
			changed = true
		case "oss-url":
			cfg.OssURL = stringPtr(f.Value.String())
			changed = true
		case "api-key":
			cfg.APIKey = stringPtr(f.Value.String())
			changed = true
		case "username":
			cfg.Username = stringPtr(f.Value.String())
			changed = true
		case "password":
			cfg.Password = stringPtr(f.Value.String())
			changed = true
		case "system":
			cfg.System = stringPtr(f.Value.String())
			changed = true
		case "stream":
			if value, err := strconv.ParseBool(f.Value.String()); err == nil {
				cfg.Stream = boolPtr(value)
				changed = true
			}
		case "file-format":
			cfg.FileFormat = stringPtr(f.Value.String())
			changed = true
		case "save-images":
			cfg.SaveImages = stringPtr(f.Value.String())
			changed = true
		}
	})
	if !changed {
		return nil
	}
	return saveAppConfig(cfg)
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
