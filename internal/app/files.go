package app

import (
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type textFileContent struct {
	Path string
	Text string
}

func collectInputAttachments(prompt string, explicit []string) ([]string, []textFileContent, error) {
	seen := make(map[string]struct{})
	var paths []string
	for _, path := range explicit {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		paths = appendUniquePath(paths, seen, path)
	}
	for _, path := range atFileRefs(prompt) {
		paths = appendUniquePath(paths, seen, path)
	}

	var images []string
	var files []textFileContent
	for _, path := range paths {
		if ok, err := isImageInput(path); err != nil {
			return nil, nil, err
		} else if ok {
			images = append(images, path)
			continue
		}
		text, err := readTextFile(path)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, textFileContent{Path: path, Text: text})
	}
	return images, files, nil
}

func collectTextFiles(prompt string, explicit []string) ([]textFileContent, error) {
	_, files, err := collectInputAttachments(prompt, explicit)
	return files, err
}

func appendUniquePath(paths []string, seen map[string]struct{}, path string) []string {
	key := path
	if !isHTTPURL(path) && !strings.HasPrefix(path, "data:image/") {
		key = filepath.Clean(path)
		if abs, err := filepath.Abs(key); err == nil {
			key = abs
		}
	}
	if _, ok := seen[key]; ok {
		return paths
	}
	seen[key] = struct{}{}
	return append(paths, path)
}

func readTextFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("read file %q: is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	if !isTextData(path, data) {
		return "", fmt.Errorf("read file %q: not a text file", path)
	}
	return strings.TrimPrefix(string(data), "\ufeff"), nil
}

func isTextData(path string, data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if bytes.Contains(data, []byte{0}) || !utf8.Valid(data) {
		return false
	}
	mediaType := mime.TypeByExtension(filepath.Ext(path))
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if mediaType != "" && isKnownTextExtension(filepath.Ext(path)) {
		return true
	}
	detected := http.DetectContentType(data)
	return strings.HasPrefix(detected, "text/")
}

func isImageInput(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	if strings.HasPrefix(value, "data:image/") || isLikelyImageURL(value) {
		return true, nil
	}
	if isHTTPURL(value) {
		return false, fmt.Errorf("input URL %q is not a recognized image URL", value)
	}

	info, err := os.Stat(value)
	if err != nil {
		return false, nil
	}
	if info.IsDir() {
		return false, nil
	}
	data, err := os.ReadFile(value)
	if err != nil {
		return false, fmt.Errorf("read file %q: %w", value, err)
	}
	return isImageData(value, data), nil
}

func isImageData(path string, data []byte) bool {
	mediaType := mime.TypeByExtension(filepath.Ext(path))
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	if len(data) == 0 {
		return false
	}
	detected := http.DetectContentType(data)
	return strings.HasPrefix(detected, "image/")
}

func isLikelyImageURL(value string) bool {
	if !isHTTPURL(value) {
		return false
	}
	mediaType := mime.TypeByExtension(filepath.Ext(pathFromURL(value)))
	return strings.HasPrefix(mediaType, "image/")
}

func isKnownTextExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".c", ".cc", ".cpp", ".cs", ".css", ".csv", ".go", ".h", ".hpp", ".htm", ".html", ".ini", ".java", ".js", ".json", ".jsx", ".log", ".md", ".php", ".ps1", ".py", ".rb", ".rs", ".sh", ".sql", ".svg", ".toml", ".ts", ".tsx", ".txt", ".xml", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func formatTextFileContent(file textFileContent) string {
	return fmt.Sprintf("File: %s\n\n%s", file.Path, file.Text)
}

func atFileRefs(prompt string) []string {
	var refs []string
	runes := []rune(prompt)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '@' {
			continue
		}
		if i > 0 && isPathChar(runes[i-1]) {
			continue
		}
		path, end := readAtPath(runes, i+1)
		i = end
		if path == "" || isHTTPURL(path) {
			if isLikelyImageURL(path) {
				refs = append(refs, path)
			}
			continue
		}
		if existing := existingPathCandidate(path); existing != "" {
			refs = append(refs, existing)
		}
	}
	return refs
}

func readAtPath(runes []rune, start int) (string, int) {
	if start >= len(runes) {
		return "", start
	}
	if runes[start] == '"' || runes[start] == '\'' {
		quote := runes[start]
		for i := start + 1; i < len(runes); i++ {
			if runes[i] == quote {
				return string(runes[start+1 : i]), i
			}
		}
		return "", start
	}
	end := start
	for end < len(runes) && !unicode.IsSpace(runes[end]) {
		end++
	}
	return string(runes[start:end]), end - 1
}

func existingPathCandidate(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "\"'"))
	for value != "" {
		candidate := strings.TrimRight(value, ",.;:，。；：)]}）】")
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		next := strings.TrimRightFunc(candidate, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSymbol(r)
		})
		if next == candidate {
			return ""
		}
		value = next
	}
	return ""
}

func isPathChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}
