package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

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
