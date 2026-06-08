package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	fileFormatBase64 = "base64"
	fileFormatURL    = "url"
)

type inputMessage struct {
	Role    string         `json:"role"`
	Content []inputContent `json:"content"`
}

type inputContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

func buildInputWithFileFormat(ctx context.Context, c *client, prompt, system string, images []string, files []textFileContent, fileFormat string) ([]inputMessage, error) {
	fileFormat, err := normalizeFileFormat(fileFormat)
	if err != nil {
		return nil, err
	}
	var messages []inputMessage
	if strings.TrimSpace(system) != "" {
		messages = append(messages, inputMessage{
			Role: "system",
			Content: []inputContent{{
				Type: "input_text",
				Text: strings.TrimSpace(system),
			}},
		})
	}

	content := []inputContent{}
	if prompt != "" {
		content = append(content, inputContent{
			Type: "input_text",
			Text: prompt,
		})
	}
	for _, file := range files {
		content = append(content, inputContent{
			Type: "input_text",
			Text: formatTextFileContent(file),
		})
	}
	for _, image := range images {
		imageURL, err := imageInputURL(ctx, c, image, fileFormat)
		if err != nil {
			return nil, err
		}
		content = append(content, inputContent{
			Type:     "input_image",
			ImageURL: imageURL,
		})
	}
	messages = append(messages, inputMessage{Role: "user", Content: content})
	return messages, nil
}

func normalizeFileFormat(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", fileFormatBase64:
		return fileFormatBase64, nil
	case fileFormatURL:
		return fileFormatURL, nil
	default:
		return "", fmt.Errorf("invalid -file-format %q: expected base64 or url", value)
	}
}

func imageInputURL(ctx context.Context, c *client, value, fileFormat string) (string, error) {
	value = strings.TrimSpace(value)
	if isHTTPURL(value) || strings.HasPrefix(value, "data:image/") {
		return value, nil
	}
	if fileFormat == fileFormatURL {
		if c != nil {
			if uploaded, err := c.uploadLocalFile(ctx, value); err == nil && strings.TrimSpace(uploaded) != "" {
				return uploaded, nil
			}
		}
	}
	return imageDataURLFromFile(value)
}

func imageDataURLFromFile(value string) (string, error) {
	data, err := os.ReadFile(value)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", value, err)
	}
	mediaType := mime.TypeByExtension(filepath.Ext(value))
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	return imageDataURL(mediaType, data), nil
}

func imageDataURL(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}
