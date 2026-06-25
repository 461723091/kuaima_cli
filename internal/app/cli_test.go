package app

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

func TestImageInputURLFallsBackToBase64WhenUploadFails(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "photo.png")
	writeTestFile(t, imagePath, tinyPNG())

	got, err := imageInputURL(context.Background(), nil, imagePath, fileFormatURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 11 || got[:11] != "data:image/" {
		t.Fatalf("expected data url fallback, got %q", got)
	}
}

func TestImageCommandSharedFlagsParse(t *testing.T) {
	fs := newFlagSet("image")
	opts := addClientFlags(fs)
	stream := fs.Bool("stream", true, "stream image generation events")
	saveDir := addSaveImagesFlag(fs, ".")
	inputOpts := addInputFlags(fs)
	imageOpts := addImageFlags(fs, appConfig{})

	err := parseFlags(fs, []string{
		"-image-model", "image-model",
		"-image-size", "1536x1024",
		"-image-quality", "high",
		"-image-count", "3",
		"-image-output-format", "webp",
		"-image-output-compression", "80",
		"-image-background", "transparent",
		"-image-moderation", "low",
		"-image-action", "edit",
		"-stream",
		"-file-format", "url",
		"-file", "https://example.com/reference.png",
		"-image-mask", "https://example.com/mask.png",
		"-save-images", "outputs",
		"generate a variant",
	})
	if err != nil {
		t.Fatal(err)
	}

	if opts.imageGenerationModel() != "image-model" {
		t.Fatalf("unexpected image model: %q", opts.imageGenerationModel())
	}
	if !*stream {
		t.Fatal("expected stream to be true")
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
	if imageOpts.mask == nil || *imageOpts.mask != "https://example.com/mask.png" {
		t.Fatalf("unexpected image mask: %#v", imageOpts.mask)
	}
	if imageOpts.upscale == nil || !*imageOpts.upscale {
		t.Fatalf("expected image upscale to default on, got %#v", imageOpts.upscale)
	}
	if err := imageOpts.validate(); err != nil {
		t.Fatal(err)
	}
	req := imageOpts.generationRequest(opts.imageGenerationModel(), "generate a variant")
	if req.Model != "image-model" || req.Size != "1536x1024" || req.Quality != "high" || req.N != 3 || req.OutputFormat != "webp" || req.OutputCompression == nil || *req.OutputCompression != 80 || req.Background != "transparent" || req.Moderation != "low" {
		t.Fatalf("unexpected image request: %#v", req)
	}
	tool := imageOpts.responseTool()
	if tool["action"] != "edit" || tool["size"] != "1536x1024" || tool["quality"] != "high" || tool["output_format"] != "webp" || tool["output_compression"] != 80 || tool["background"] != "transparent" {
		t.Fatalf("unexpected response tool: %#v", tool)
	}
	if imageOpts.responseRunCount() != 3 {
		t.Fatalf("unexpected response run count: %d", imageOpts.responseRunCount())
	}
	if _, ok := tool["moderation"]; ok {
		t.Fatalf("did not expect moderation in response tool: %#v", tool)
	}
	editReq := imageOpts.editRequest(opts.imageGenerationModel(), "generate a variant", []imageRef{{ImageURL: "https://example.com/reference.png"}}, &imageRef{ImageURL: *imageOpts.mask})
	if editReq.Model != "image-model" || editReq.Prompt != "generate a variant" || len(editReq.Images) != 1 || editReq.Mask == nil || editReq.Mask.ImageURL != "https://example.com/mask.png" || editReq.N != 3 || editReq.Size != "1536x1024" || editReq.OutputCompression == nil || *editReq.OutputCompression != 80 {
		t.Fatalf("unexpected image edit request: %#v", editReq)
	}
}

func TestImageStreamDefaultsToTrue(t *testing.T) {
	fs := newFlagSet("image")
	stream := fs.Bool("stream", configBool(appConfig{}.Stream, true), "stream image generation events")
	if err := parseFlags(fs, nil); err != nil {
		t.Fatal(err)
	}
	if !*stream {
		t.Fatal("expected image stream to default to true")
	}
}

func TestImageGenerationModelDefaultsToGPTImage2(t *testing.T) {
	fs := newFlagSet("image")
	opts := addClientFlags(fs)
	if err := parseFlags(fs, []string{"-model", "cli-model"}); err != nil {
		t.Fatal(err)
	}
	if opts.imageGenerationModel() != defaultImageModel {
		t.Fatalf("expected default image model, got %q", opts.imageGenerationModel())
	}
}

func TestImageCommandDoesNotAcceptOutputFlag(t *testing.T) {
	fs := newFlagSet("image")
	_ = addClientFlags(fs)
	_ = addSaveImagesFlag(fs, ".")
	_ = addInputFlags(fs)
	_ = addImageFlags(fs, appConfig{})

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
