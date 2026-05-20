package main

import (
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
