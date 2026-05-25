package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	imageURLPattern = regexp.MustCompile(`(?i)(?:https?://[^\s"'<>]+\.(?:png|jpe?g|webp|gif|bmp|svg)(?:\?[^\s"'<>]*)?)`)
	dataURLPattern  = regexp.MustCompile(`(?is)data:image/[a-z0-9.+-]+;base64,[a-z0-9+/=\r\n]+`)
)

type imageCandidate struct {
	Kind      string
	Value     string
	MediaType string
}

type responseImageSaver struct {
	ctx        context.Context
	httpClient *http.Client
	dir        string
	seen       map[string]bool
	saved      []string
}

func newResponseImageSaver(ctx context.Context, httpClient *http.Client, dir string) *responseImageSaver {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	return &responseImageSaver{
		ctx:        ctx,
		httpClient: httpClient,
		dir:        dir,
		seen:       map[string]bool{},
	}
}

func saveImagesFromResponse(ctx context.Context, httpClient *http.Client, resp *responsePayload, dir string) ([]string, error) {
	return newResponseImageSaver(ctx, httpClient, dir).saveResponse(resp)
}

func (s *responseImageSaver) saveResponse(resp *responsePayload) ([]string, error) {
	return s.saveCandidates(imageCandidates(resp))
}

func (s *responseImageSaver) saveCandidates(candidates []imageCandidate) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	var saved []string
	for _, candidate := range candidates {
		path, err := s.saveCandidate(candidate)
		if err != nil {
			return saved, err
		}
		if path != "" {
			saved = append(saved, path)
		}
	}
	return saved, nil
}

func (s *responseImageSaver) saveCandidate(candidate imageCandidate) (string, error) {
	if s == nil {
		return "", nil
	}
	if strings.TrimSpace(candidate.Value) == "" {
		return "", nil
	}
	if s.seen[candidate.Value] {
		return "", nil
	}
	s.seen[candidate.Value] = true

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return "", err
	}
	data, mediaType, err := loadImageCandidate(s.ctx, s.httpClient, candidate)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("response-image-%02d%s", len(s.saved)+1, extensionForMediaType(mediaType, candidate.Value))
	path := filepath.Join(s.dir, name)
	path, err = writeFileUnique(path, data, 0644)
	if err != nil {
		return "", err
	}
	s.saved = append(s.saved, path)
	return path, nil
}

func writeFileUnique(path string, data []byte, perm os.FileMode) (string, error) {
	for i := 0; ; i++ {
		candidate := numberedPath(path, i)
		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := file.Write(data); err != nil {
			closeErr := file.Close()
			if closeErr != nil {
				return "", fmt.Errorf("%w; close file: %v", err, closeErr)
			}
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		return candidate, nil
	}
}

func numberedPath(path string, index int) string {
	if index == 0 {
		return path
	}
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	return fmt.Sprintf("%s-%02d%s", stem, index, ext)
}

func imageCandidates(resp *responsePayload) []imageCandidate {
	var candidates []imageCandidate
	for _, out := range resp.Output {
		if out.Type == "image_generation_call" && strings.TrimSpace(out.Result) != "" {
			candidates = append(candidates, imageCandidate{Kind: "base64", Value: out.Result})
		}
		for _, content := range out.Content {
			if strings.TrimSpace(content.ImageURL) != "" {
				candidates = append(candidates, classifyImageValue(content.ImageURL))
			}
			candidates = append(candidates, candidatesFromText(content.Text)...)
		}
	}
	candidates = append(candidates, candidatesFromText(resp.OutputText)...)
	if len(resp.Raw) > 0 {
		candidates = append(candidates, candidatesFromText(string(resp.Raw))...)
		candidates = append(candidates, candidatesFromJSON(resp.Raw)...)
	}
	return candidates
}

func candidatesFromJSON(data []byte) []imageCandidate {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	var candidates []imageCandidate
	walkJSON(value, "", &candidates)
	return candidates
}

func walkJSON(value any, key string, candidates *[]imageCandidate) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			walkJSON(childValue, childKey, candidates)
		}
	case []any:
		for _, childValue := range typed {
			walkJSON(childValue, key, candidates)
		}
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return
		}
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(text, "data:image/") {
			*candidates = append(*candidates, classifyImageValue(text))
			return
		}
		if isHTTPURL(text) && (strings.Contains(lowerKey, "image") || strings.Contains(lowerKey, "url")) {
			*candidates = append(*candidates, imageCandidate{Kind: "url", Value: text})
			return
		}
		if looksLikeImageBase64Key(lowerKey) && looksLikeBase64(text) {
			*candidates = append(*candidates, imageCandidate{Kind: "base64", Value: text})
		}
	}
}

func looksLikeImageBase64Key(key string) bool {
	return strings.Contains(key, "b64") ||
		strings.Contains(key, "base64") ||
		strings.Contains(key, "image") ||
		key == "result"
}

func looksLikeBase64(value string) bool {
	cleaned := cleanBase64(value)
	if len(cleaned) < 128 || len(cleaned)%4 != 0 {
		return false
	}
	for _, r := range cleaned {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
			continue
		}
		return false
	}
	return true
}

func candidatesFromText(text string) []imageCandidate {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var candidates []imageCandidate
	for _, match := range dataURLPattern.FindAllString(text, -1) {
		candidates = append(candidates, classifyImageValue(match))
	}
	for _, match := range imageURLPattern.FindAllString(text, -1) {
		candidates = append(candidates, imageCandidate{Kind: "url", Value: strings.TrimRight(match, ".,)")})
	}
	return candidates
}

func classifyImageValue(value string) imageCandidate {
	if strings.HasPrefix(value, "data:image/") {
		mediaType, _, _ := strings.Cut(strings.TrimPrefix(value, "data:"), ";")
		return imageCandidate{Kind: "data", Value: value, MediaType: mediaType}
	}
	if isHTTPURL(value) {
		return imageCandidate{Kind: "url", Value: value}
	}
	return imageCandidate{Kind: "base64", Value: value}
}

func loadImageCandidate(ctx context.Context, httpClient *http.Client, candidate imageCandidate) ([]byte, string, error) {
	switch candidate.Kind {
	case "url":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.Value, nil)
		if err != nil {
			return nil, "", err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("download image failed: %s: %s", candidate.Value, resp.Status)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}
		mediaType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
		if mediaType == "" {
			mediaType = http.DetectContentType(data)
		}
		return data, mediaType, nil
	case "data":
		meta, data, ok := strings.Cut(candidate.Value, ",")
		if !ok {
			return nil, "", errors.New("invalid data URL image result")
		}
		mediaType := strings.TrimPrefix(strings.TrimSuffix(meta, ";base64"), "data:")
		bytes, err := base64.StdEncoding.DecodeString(cleanBase64(data))
		return bytes, mediaType, err
	default:
		bytes, err := base64.StdEncoding.DecodeString(cleanBase64(candidate.Value))
		if err != nil {
			return nil, "", fmt.Errorf("decode image: %w", err)
		}
		mediaType := candidate.MediaType
		if mediaType == "" {
			mediaType = http.DetectContentType(bytes)
		}
		return bytes, mediaType, nil
	}
}

func extensionForMediaType(mediaType, source string) string {
	if isHTTPURL(source) {
		if ext := filepath.Ext(pathFromURL(source)); ext != "" {
			return ext
		}
	}
	if mediaType != "" {
		if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
			return exts[0]
		}
	}
	return ".png"
}

func pathFromURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Path
}
