package main

import (
	"bytes"
	"io"
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

func TestCreateResponseStreamAggregatesOutputItemEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.image_generation_call.partial_image\",\"output_index\":0,\"partial_image_b64\":\"partial-image\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"ig_1\",\"type\":\"image_generation_call\",\"status\":\"in_progress\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"id\":\"ig_1\",\"type\":\"image_generation_call\",\"status\":\"completed\",\"result\":\"final-image\"}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	c, err := newClient(server.URL, "", "secret-token", "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var out bytes.Buffer
	resp, err := c.createResponseStream(t.Context(), responseRequest{Model: "test-model", Input: "draw"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	item := resp.Output[0]
	if item.ID != "ig_1" || item.Type != "image_generation_call" || item.Status != "completed" || item.Result != "final-image" {
		t.Fatalf("unexpected output item: %+v", item)
	}
	if !strings.Contains(string(resp.Raw), "response.image_generation_call.partial_image") {
		t.Fatalf("raw stream missing partial image event: %s", string(resp.Raw))
	}
}

func TestCreateResponseStreamKeepsOutputItemsWhenCompletedResponseIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"ig_1\",\"type\":\"image_generation_call\",\"status\":\"in_progress\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.image_generation_call.partial_image\",\"output_index\":0,\"partial_image_b64\":\"partial-image\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"id\":\"ig_1\",\"type\":\"image_generation_call\",\"status\":\"generating\",\"result\":\"final-image\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"output\":[]}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	c, err := newClient(server.URL, "", "secret-token", "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	resp, err := c.createResponseStream(t.Context(), responseRequest{Model: "test-model", Input: "draw"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected completed response to keep streamed output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Result != "final-image" {
		t.Fatalf("unexpected image result: %+v", resp.Output[0])
	}
}

func TestCreateResponseStreamKeepsPartialImageWithoutDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"ig_1\",\"type\":\"image_generation_call\",\"status\":\"in_progress\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.image_generation_call.partial_image\",\"output_index\":0,\"partial_image_b64\":\"partial-image\"}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	c, err := newClient(server.URL, "", "secret-token", "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	resp, err := c.createResponseStream(t.Context(), responseRequest{Model: "test-model", Input: "draw"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	if resp.Output[0].Result != "partial-image" {
		t.Fatalf("unexpected partial image result: %+v", resp.Output[0])
	}
}
