package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileUniqueRenamesExistingPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.png")
	if err := os.WriteFile(path, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}

	saved, err := writeFileUnique(path, []byte("second"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "image-01.png")
	if saved != want {
		t.Fatalf("expected %q, got %q", want, saved)
	}

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "first" {
		t.Fatalf("original file was overwritten: %q", original)
	}

	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second" {
		t.Fatalf("unexpected saved file data: %q", data)
	}
}

func TestSaveResponseImagesWithNilSaverSkipsSaving(t *testing.T) {
	saved, err := saveResponseImagesWithSaver(nil, &responsePayload{})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 0 {
		t.Fatalf("expected no saved images, got %#v", saved)
	}
}

func TestResponseImageSaverUpscalesSmallPNG(t *testing.T) {
	dir := t.TempDir()
	saver := newResponseImageSaver(context.Background(), nil, dir, true, "4x4")
	if saver == nil {
		t.Fatal("expected saver")
	}

	path, err := saver.saveCandidate(imageCandidate{Kind: "base64", Value: base64.StdEncoding.EncodeToString(tinyPNG())})
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected saved path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 4 || bounds.Dy() != 4 {
		t.Fatalf("expected 4x4 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
