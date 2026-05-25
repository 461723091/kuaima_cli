package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
