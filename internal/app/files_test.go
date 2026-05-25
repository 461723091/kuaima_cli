package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectTextFilesExplicitAndAtRefs(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.md")
	if err := os.WriteFile(first, []byte("first body"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("# second"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := collectTextFiles("please read @"+second+" and @"+second, []string{first})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != first || files[0].Text != "first body" {
		t.Fatalf("unexpected first file: %#v", files[0])
	}
	if files[1].Path != second || files[1].Text != "# second" {
		t.Fatalf("unexpected second file: %#v", files[1])
	}
}

func TestCollectTextFilesQuotedAtRef(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "with space.txt")
	if err := os.WriteFile(path, []byte("quoted body"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := collectTextFiles(`summarize @"`+path+`"`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != path || files[0].Text != "quoted body" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestCollectTextFilesRejectsBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := collectTextFiles("", []string{path}); err == nil {
		t.Fatal("expected binary file error")
	}
}

func TestCollectInputAttachmentsClassifiesExplicitFiles(t *testing.T) {
	dir := t.TempDir()
	textPath := filepath.Join(dir, "notes.txt")
	imagePath := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(textPath, []byte("notes body"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imagePath, tinyPNG(), 0644); err != nil {
		t.Fatal(err)
	}

	images, files, err := collectInputAttachments("", []string{textPath, imagePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0] != imagePath {
		t.Fatalf("unexpected images: %#v", images)
	}
	if len(files) != 1 || files[0].Path != textPath || files[0].Text != "notes body" {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestCollectInputAttachmentsClassifiesAtImageRef(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(imagePath, tinyPNG(), 0644); err != nil {
		t.Fatal(err)
	}

	images, files, err := collectInputAttachments("describe @"+imagePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0] != imagePath {
		t.Fatalf("unexpected images: %#v", images)
	}
	if len(files) != 0 {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89,
	}
}
