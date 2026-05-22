package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsHaveLowestUserPriority(t *testing.T) {
	t.Setenv("KUAIMA_MODEL", "env-model")
	t.Setenv("KUAIMA_IMAGE_MODEL", "env-image-model")
	t.Setenv("KUAIMA_BASE_URL", "")
	t.Setenv("KUAIMA_OSS_URL", "")
	t.Setenv("KUAIMA_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	cfg := appConfig{
		Model:                  stringPtr("config-model"),
		ImageModel:             stringPtr("config-image-model"),
		ImageSize:              stringPtr("1024x1024"),
		ImageQuality:           stringPtr("medium"),
		ImageCount:             intPtr(2),
		ImageOutputFormat:      stringPtr("jpeg"),
		ImageOutputCompression: intPtr(75),
		ImageBackground:        stringPtr("opaque"),
		ImageModeration:        stringPtr("low"),
		ImageAction:            stringPtr("edit"),
		BaseURL:                stringPtr("https://config.example.com"),
		OssURL:                 stringPtr("https://oss-config.example.com"),
		APIKey:                 stringPtr("config-key"),
		System:                 stringPtr("config-system"),
		Stream:                 boolPtr(true),
		FileFormat:             stringPtr(fileFormatURL),
		SaveImages:             stringPtr("config-images"),
	}
	fs := newFlagSet("ask")
	opts := addResponseFlagsWithConfig(fs, cfg)
	saveDir := addSaveImagesFlagWithConfig(fs, cfg, ".")
	inputOpts := addInputFlagsWithConfig(fs, cfg)
	imageOpts := addImageFlags(fs, cfg)

	if err := parseFlags(fs, []string{"-base-url", "https://cli.example.com"}); err != nil {
		t.Fatal(err)
	}

	if *opts.model != "env-model" {
		t.Fatalf("expected env model, got %q", *opts.model)
	}
	if *opts.imageModel != "env-image-model" {
		t.Fatalf("expected env image model, got %q", *opts.imageModel)
	}
	if *imageOpts.size != "1024x1024" || *imageOpts.quality != "medium" || *imageOpts.count != 2 || *imageOpts.outputFormat != "jpeg" || *imageOpts.outputCompression != 75 || *imageOpts.background != "opaque" || *imageOpts.moderation != "low" || *imageOpts.action != "edit" {
		t.Fatalf("unexpected image options: %#v", imageOpts)
	}
	if *opts.baseURL != "https://cli.example.com" {
		t.Fatalf("expected CLI base URL, got %q", *opts.baseURL)
	}
	if *opts.ossURL != "https://oss-config.example.com" {
		t.Fatalf("expected config OSS URL, got %q", *opts.ossURL)
	}
	if *opts.apiKey != "config-key" {
		t.Fatalf("expected config API key, got %q", *opts.apiKey)
	}
	if *opts.system != "config-system" {
		t.Fatalf("expected config system, got %q", *opts.system)
	}
	if !*opts.stream {
		t.Fatal("expected config stream to be true")
	}
	if *inputOpts.fileFormat != fileFormatURL {
		t.Fatalf("expected config file format, got %q", *inputOpts.fileFormat)
	}
	if *saveDir != "config-images" {
		t.Fatalf("expected config save dir, got %q", *saveDir)
	}
}

func TestPersistConfigFlagsUpdatesOnlyExplicitPersistentFlags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUAIMA_CONFIG_DIR", dir)

	cfg := appConfig{
		Model:   stringPtr("old-model"),
		BaseURL: stringPtr("https://old.example.com"),
	}
	fs := newFlagSet("ask")
	_ = addResponseFlagsWithConfig(fs, cfg)
	_ = addSaveImagesFlagWithConfig(fs, cfg, ".")
	_ = addInputFlagsWithConfig(fs, cfg)
	_ = addImageFlags(fs, cfg)
	if err := parseFlags(fs, []string{
		"-model", "new-model",
		"-image-model", "new-image-model",
		"-image-size", "1536x1024",
		"-image-quality", "high",
		"-image-count", "4",
		"-image-output-format", "webp",
		"-image-output-compression", "60",
		"-image-background", "transparent",
		"-image-moderation", "low",
		"-image-action", "edit",
		"-stream=false",
		"-file-format", fileFormatURL,
		"-save-images", "",
		"-file", "ignored.txt",
	}); err != nil {
		t.Fatal(err)
	}
	if err := persistConfigFlags(fs, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got["model"] != "new-model" {
		t.Fatalf("expected updated model, got %#v", got["model"])
	}
	if got["image_model"] != "new-image-model" {
		t.Fatalf("expected updated image_model, got %#v", got["image_model"])
	}
	if got["image_size"] != "1536x1024" || got["image_quality"] != "high" || got["image_count"] != float64(4) || got["image_output_format"] != "webp" || got["image_output_compression"] != float64(60) || got["image_background"] != "transparent" || got["image_moderation"] != "low" || got["image_action"] != "edit" {
		t.Fatalf("unexpected persisted image options: %#v", got)
	}
	if got["base_url"] != "https://old.example.com" {
		t.Fatalf("expected existing base_url to be preserved, got %#v", got["base_url"])
	}
	if got["stream"] != false {
		t.Fatalf("expected explicit false stream, got %#v", got["stream"])
	}
	if got["file_format"] != fileFormatURL {
		t.Fatalf("expected file format, got %#v", got["file_format"])
	}
	if got["save_images"] != "" {
		t.Fatalf("expected empty save_images, got %#v", got["save_images"])
	}
	if _, ok := got["file"]; ok {
		t.Fatal("did not expect file attachments to be persisted")
	}
}

func TestConfigSaveImagesAllowsEmptyValue(t *testing.T) {
	cfg := appConfig{SaveImages: stringPtr("")}
	fs := newFlagSet("image")
	saveDir := addSaveImagesFlagWithConfig(fs, cfg, ".")
	if err := parseFlags(fs, nil); err != nil {
		t.Fatal(err)
	}
	if *saveDir != "" {
		t.Fatalf("expected empty save-images from config, got %q", *saveDir)
	}
}

func TestLoadAppConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUAIMA_CONFIG_DIR", dir)
	cfg, err := loadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != nil {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}\n" {
		t.Fatalf("expected empty config file, got %q", string(data))
	}
}

func TestLoadAppConfigReadsOldConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUAIMA_CONFIG_DIR", dir)
	writeTestFile(t, filepath.Join(dir, "conf.json"), []byte(`{"api_key":"old-key"}`))

	cfg, err := loadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if configString(cfg.APIKey, "") != "old-key" {
		t.Fatalf("expected old config api key, got %#v", cfg.APIKey)
	}
}
