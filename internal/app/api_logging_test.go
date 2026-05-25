package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrettyJSONTruncatesLongBase64Fields(t *testing.T) {
	long := strings.Repeat("a", maxLoggedLongString+100)
	got := prettyJSON([]byte(`{"output":[{"type":"image_generation_call","result":"` + long + `"}],"message":"ok"}`))

	if strings.Contains(got, long) {
		t.Fatalf("log still contains full base64 field: %s", got)
	}
	if !strings.Contains(got, "...<truncated 100 chars>") {
		t.Fatalf("log missing truncation marker: %s", got)
	}
	if !strings.Contains(got, `"message": "ok"`) {
		t.Fatalf("log should keep short fields: %s", got)
	}
}

func TestPrettyJSONTruncatesDataURLs(t *testing.T) {
	longURL := "data:image/png;base64," + strings.Repeat("a", maxLoggedLongString+100)
	got := prettyJSON([]byte(`{"input_image":{"image_url":"` + longURL + `"}}`))

	if strings.Contains(got, longURL) {
		t.Fatalf("log still contains full data URL: %s", got)
	}
	if !strings.Contains(got, "...<truncated") {
		t.Fatalf("log missing truncation marker: %s", got)
	}
}

func TestPrettyJSONTruncatesEventStreamJSON(t *testing.T) {
	long := strings.Repeat("a", maxLoggedLongString+100)
	got := prettyJSON([]byte("data: {\"type\":\"image_generation.completed\",\"b64_json\":\"" + long + "\"}\n\ndata: [DONE]\n\n"))

	if strings.Contains(got, long) {
		t.Fatalf("stream log still contains full base64 field: %s", got)
	}
	if !strings.Contains(got, "data: {") || !strings.Contains(got, "...<truncated 100 chars>") {
		t.Fatalf("unexpected stream log: %s", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("stream log should keep DONE marker: %s", got)
	}

	line := strings.TrimSpace(strings.Split(got, "\n")[0])
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("truncated stream payload should remain JSON: %v\n%s", err, got)
	}
}
