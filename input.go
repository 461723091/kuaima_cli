package main

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func buildInput(prompt, system string, images []string) ([]inputMessage, error) {
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

	content := []inputContent{{
		Type: "input_text",
		Text: prompt,
	}}
	for _, image := range images {
		imageURL, err := imageInputURL(image)
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

func imageInputURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if isHTTPURL(value) || strings.HasPrefix(value, "data:image/") {
		return value, nil
	}

	data, err := os.ReadFile(value)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", value, err)
	}
	mediaType := mime.TypeByExtension(filepath.Ext(value))
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
