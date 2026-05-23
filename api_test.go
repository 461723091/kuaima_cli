package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCreateResponseWritesLogFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"id":"resp_1","output_text":"ok"}`))
	}))
	defer server.Close()

	logPath := filepath.Join(t.TempDir(), "kuaima.log")
	c, err := newClient(server.URL, "", "secret-token", "", "", false, logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	resp, err := c.createResponse(t.Context(), responseRequest{
		Model: "test-model",
		Input: []inputMessage{{Role: "user", Content: []inputContent{{Type: "input_text", Text: "hello"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.text() != "ok" {
		t.Fatalf("unexpected response text: %q", resp.text())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, want := range []string{"--- request ---", "POST " + server.URL + "/v1/responses", "Authorization: Bearer <redacted>", "--- response ---", "200 OK", `"output_text": "ok"`} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q:\n%s", want, log)
		}
	}
	prefixPattern := regexp.MustCompile(`(?m)^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\] \[kuaima_cli\.\(\*client\)\.createResponse\] --- (request|response|timing) ---$`)
	if matches := prefixPattern.FindAllString(log, -1); len(matches) != 3 {
		t.Fatalf("expected request/response/timing prefixes, got %d:\n%s", len(matches), log)
	}
	if strings.Contains(log, "secret-token") {
		t.Fatalf("log should redact token:\n%s", log)
	}
}
