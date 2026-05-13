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

func saveImagesFromResponse(ctx context.Context, httpClient *http.Client, resp *responsePayload, dir string) ([]string, error) {
	candidates := imageCandidates(resp)
	if len(candidates) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var saved []string
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if seen[candidate.Value] {
			continue
		}
		seen[candidate.Value] = true

		data, mediaType, err := loadImageCandidate(ctx, httpClient, candidate)
		if err != nil {
			return saved, err
		}
		name := fmt.Sprintf("response-image-%02d%s", len(saved)+1, extensionForMediaType(mediaType, candidate.Value))
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			return saved, err
		}
		saved = append(saved, path)
	}
	return saved, nil
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
