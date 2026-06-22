package app

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

const (
	configDirName  = ".kuaima"
	configFileName = "config.json"
)

var (
	executablePath       = os.Executable
	userHomeDir          = os.UserHomeDir
	preferHomeConfigPath bool
)

type appConfig struct {
	Model                  *string `json:"model,omitempty"`
	ImageModel             *string `json:"image_model,omitempty"`
	ImageSize              *string `json:"image_size,omitempty"`
	ImageQuality           *string `json:"image_quality,omitempty"`
	ImageCount             *int    `json:"image_count,omitempty"`
	ImageOutputFormat      *string `json:"image_output_format,omitempty"`
	ImageOutputCompression *int    `json:"image_output_compression,omitempty"`
	ImageBackground        *string `json:"image_background,omitempty"`
	ImageModeration        *string `json:"image_moderation,omitempty"`
	ImageAction            *string `json:"image_action,omitempty"`
	BaseURL                *string `json:"base_url,omitempty"`
	OssURL                 *string `json:"oss_url,omitempty"`
	APIKey                 *string `json:"api_key,omitempty"`
	Username               *string `json:"username,omitempty"`
	Password               *string `json:"password,omitempty"`
	System                 *string `json:"system,omitempty"`
	Stream                 *bool   `json:"stream,omitempty"`
	FileFormat             *string `json:"file_format,omitempty"`
	SaveImages             *string `json:"save_images,omitempty"`
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
	if err := ensureAppConfigDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := marshalAppConfigPreservingExtraFields(path, cfg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func marshalAppConfigPreservingExtraFields(path string, cfg appConfig) ([]byte, error) {
	merged := map[string]json.RawMessage{}
	existingData, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if strings.TrimSpace(string(existingData)) != "" {
		if err := json.Unmarshal(existingData, &merged); err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
	}

	cfgData, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var cfgFields map[string]json.RawMessage
	if err := json.Unmarshal(cfgData, &cfgFields); err != nil {
		return nil, err
	}

	for _, key := range appConfigJSONFields() {
		delete(merged, key)
	}
	for key, value := range cfgFields {
		merged[key] = value
	}
	return json.MarshalIndent(merged, "", "  ")
}

func appConfigJSONFields() []string {
	return []string{
		"model",
		"image_model",
		"image_size",
		"image_quality",
		"image_count",
		"image_output_format",
		"image_output_compression",
		"image_background",
		"image_moderation",
		"image_action",
		"base_url",
		"oss_url",
		"api_key",
		"username",
		"password",
		"system",
		"stream",
		"file_format",
		"save_images",
	}
}

// webui默认生图保存路径
func defaultOutputPath() string {
	home, _ := userHomeDir()

	return filepath.Join(home, "Downloads")
}

func appConfigPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("KUAIMA_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, configFileName), nil
	}

	executableConfigPath, executableConfigDir := executableAppConfigPath()
	homeConfigPath, homeErr := homeAppConfigPath()
	if preferHomeConfigPath || executableLooksLikeMacBundle() {
		if homeErr == nil && fileExists(homeConfigPath) {
			_ = hideAppConfigDir(filepath.Dir(homeConfigPath))
			return homeConfigPath, nil
		}
		if executableConfigPath != "" && fileExists(executableConfigPath) {
			if err := ensureAppConfigDir(filepath.Dir(homeConfigPath)); err == nil {
				if migrated, err := migrateAppConfig(executableConfigPath, homeConfigPath); err == nil && migrated {
					return homeConfigPath, nil
				}
			}
			_ = hideAppConfigDir(executableConfigDir)
			return executableConfigPath, nil
		}
		if homeErr == nil {
			if err := ensureAppConfigDir(filepath.Dir(homeConfigPath)); err == nil {
				return homeConfigPath, nil
			}
		}
		if homeErr != nil {
			return "", homeErr
		}
		return homeConfigPath, nil
	}

	if executableConfigPath != "" && fileExists(executableConfigPath) {
		_ = hideAppConfigDir(executableConfigDir)
		return executableConfigPath, nil
	}
	if homeErr == nil && fileExists(homeConfigPath) {
		_ = hideAppConfigDir(filepath.Dir(homeConfigPath))
		return homeConfigPath, nil
	}
	if executableConfigPath != "" {
		if err := ensureAppConfigDir(executableConfigDir); err == nil {
			return executableConfigPath, nil
		}
	}
	if homeErr != nil {
		return "", homeErr
	}
	return homeConfigPath, nil
}

func executableLooksLikeMacBundle() bool {
	exePath, err := executablePath()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(filepath.ToSlash(exePath)), ".app/contents/macos/")
}

func executableAppConfigPath() (string, string) {
	exePath, err := executablePath()
	if err != nil {
		return "", ""
	}
	dir := filepath.Join(filepath.Dir(exePath), configDirName)
	return filepath.Join(dir, configFileName), dir
}

func homeAppConfigPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

func ensureAppConfigDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return hideAppConfigDir(dir)
}

func migrateAppConfig(srcPath, dstPath string) (bool, error) {
	if !fileExists(srcPath) {
		return false, nil
	}
	if fileExists(dstPath) {
		return false, nil
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return false, err
	}
	if err := ensureAppConfigDir(filepath.Dir(dstPath)); err != nil {
		return false, err
	}
	if err := os.WriteFile(dstPath, data, 0600); err != nil {
		return false, err
	}
	return true, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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

func configInt(value *int, fallback int) int {
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
		case "image-model":
			cfg.ImageModel = stringPtr(f.Value.String())
			changed = true
		case "image-size":
			cfg.ImageSize = stringPtr(f.Value.String())
			changed = true
		case "image-quality":
			cfg.ImageQuality = stringPtr(f.Value.String())
			changed = true
		case "image-count":
			if value, err := strconv.Atoi(f.Value.String()); err == nil {
				cfg.ImageCount = intPtr(value)
				changed = true
			}
		case "image-output-format":
			cfg.ImageOutputFormat = stringPtr(f.Value.String())
			changed = true
		case "image-output-compression":
			if value, err := strconv.Atoi(f.Value.String()); err == nil {
				cfg.ImageOutputCompression = intPtr(value)
				changed = true
			}
		case "image-background":
			cfg.ImageBackground = stringPtr(f.Value.String())
			changed = true
		case "image-moderation":
			cfg.ImageModeration = stringPtr(f.Value.String())
			changed = true
		case "image-action":
			cfg.ImageAction = stringPtr(f.Value.String())
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

func intPtr(value int) *int {
	return &value
}

func setPreferHomeConfigPath(prefer bool) func() {
	old := preferHomeConfigPath
	preferHomeConfigPath = prefer
	return func() {
		preferHomeConfigPath = old
	}
}
