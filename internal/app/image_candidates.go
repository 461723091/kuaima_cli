package app

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	imageURLPattern = regexp.MustCompile(`(?i)(?:https?://[^\s"'<>]+\.(?:png|jpe?g|webp|gif|bmp|svg)(?:\?[^\s"'<>]*)?)`)
	dataURLPattern  = regexp.MustCompile(`(?is)data:image/[a-z0-9.+-]+;base64,[a-z0-9+/=\r\n]+`)
)

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
