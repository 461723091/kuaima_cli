package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInputOptionsBuildInputWithImageAttachment(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "photo.png")
	writeTestFile(t, imagePath, tinyPNG())

	format := fileFormatBase64
	attachments := stringsFlag{imagePath}
	opts := inputOptions{
		fileFormat:  &format,
		attachments: &attachments,
	}

	input, err := opts.buildInput(context.Background(), nil, "edit this", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(input) != 1 || len(input[0].Content) != 2 {
		t.Fatalf("unexpected input: %#v", input)
	}
	if input[0].Content[1].Type != "input_image" || input[0].Content[1].ImageURL == "" {
		t.Fatalf("expected image input content, got %#v", input[0].Content[1])
	}
}

func TestImageCommandSharedFlagsParse(t *testing.T) {
	fs := newFlagSet("image")
	opts := addClientFlags(fs)
	saveDir := addSaveImagesFlag(fs, ".")
	inputOpts := addInputFlags(fs)

	err := parseFlags(fs, []string{
		"-image-model", "image-model",
		"-file-format", "url",
		"-file", "https://example.com/reference.png",
		"-save-images", "outputs",
		"generate a variant",
	})
	if err != nil {
		t.Fatal(err)
	}

	if opts.imageGenerationModel() != "image-model" {
		t.Fatalf("unexpected image model: %q", opts.imageGenerationModel())
	}
	if *inputOpts.fileFormat != fileFormatURL {
		t.Fatalf("unexpected file format: %q", *inputOpts.fileFormat)
	}
	if len(*inputOpts.attachments) != 1 || (*inputOpts.attachments)[0] != "https://example.com/reference.png" {
		t.Fatalf("unexpected attachments: %#v", *inputOpts.attachments)
	}
	if *saveDir != "outputs" {
		t.Fatalf("unexpected save dir: %q", *saveDir)
	}
}

func TestImageGenerationModelFallsBackToModel(t *testing.T) {
	fs := newFlagSet("image")
	opts := addClientFlags(fs)
	if err := parseFlags(fs, []string{"-model", "cli-model"}); err != nil {
		t.Fatal(err)
	}
	if opts.imageGenerationModel() != "cli-model" {
		t.Fatalf("expected image model to fall back to model, got %q", opts.imageGenerationModel())
	}
}

func TestImageCommandDoesNotAcceptOutputFlag(t *testing.T) {
	fs := newFlagSet("image")
	_ = addClientFlags(fs)
	_ = addSaveImagesFlag(fs, ".")
	_ = addInputFlags(fs)

	if err := parseFlags(fs, []string{"-o", "result.png", "generate"}); err == nil {
		t.Fatal("expected -o to be rejected")
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
