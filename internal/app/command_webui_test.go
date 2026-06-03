package app

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
)

func TestWebUIHandleGenerateUsesMultipartForImageEdits(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		var imageData []byte
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			switch part.FormName() {
			case "model", "prompt", "stream":
				fields[part.FormName()] = string(data)
			case "image":
				imageData = data
			}
		}
		if fields["model"] != "test-model" {
			t.Fatalf("unexpected model: %q", fields["model"])
		}
		if fields["prompt"] != "edit me" {
			t.Fatalf("unexpected prompt: %q", fields["prompt"])
		}
		if fields["stream"] != "" {
			t.Fatalf("unexpected stream field: %q", fields["stream"])
		}
		if string(imageData) != "fake" {
			t.Fatalf("unexpected image data: %q", string(imageData))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"image_generation_call","status":"completed","result":"ZmFrZQ=="}],"output_text":""}`))
	}))
	defer apiServer.Close()

	saveDir := t.TempDir()
	ui := &webUIServer{
		clientOpts: clientOptions{
			model:      stringPtr("gpt-5.4-mini"),
			imageModel: stringPtr("test-model"),
			baseURL:    stringPtr(apiServer.URL),
			ossURL:     stringPtr(""),
			apiKey:     stringPtr("secret-token"),
			username:   stringPtr(""),
			password:   stringPtr(""),
			verbose:    boolPtr(false),
			logFile:    stringPtr(""),
		},
		fileFormat: fileFormatBase64,
		defaults: webUIDefaults{
			ImageModel:   "test-model",
			ImageSize:    "1024x1024",
			ImageQuality: "auto",
			ImageCount:   1,
		},
		saveDir: saveDir,
	}

	body := &strings.Builder{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("prompt", "edit me")
	_ = writer.WriteField("image_model", "test-model")
	_ = writer.WriteField("image_endpoint", "image")
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="images"; filename="ref.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fake"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	ui.handleGenerate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "saved") {
		t.Fatalf("unexpected response body: %s", rr.Body.String())
	}
	if entries, err := os.ReadDir(saveDir); err != nil || len(entries) == 0 {
		t.Fatalf("expected saved image in %s, entries=%v err=%v", saveDir, entries, err)
	}
}

func TestWebUIHandleGenerateResponseUsesCLISystem(t *testing.T) {
	var gotRequest struct {
		Model  string           `json:"model"`
		Input  []inputMessage   `json:"input"`
		Tools  []map[string]any `json:"tools"`
		Stream bool             `json:"stream"`
	}
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatal(err)
		}
		if gotRequest.Model != "gpt-5.4-mini" {
			t.Fatalf("unexpected model: %q", gotRequest.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output_text":"ok","output":[]}`))
	}))
	defer apiServer.Close()

	saveDir := t.TempDir()
	ui := &webUIServer{
		clientOpts: clientOptions{
			model:      stringPtr("gpt-5.4-mini"),
			imageModel: stringPtr("test-model"),
			baseURL:    stringPtr(apiServer.URL),
			ossURL:     stringPtr(""),
			apiKey:     stringPtr("secret-token"),
			username:   stringPtr(""),
			password:   stringPtr(""),
			verbose:    boolPtr(false),
			logFile:    stringPtr(""),
		},
		responseSystem: "be concise",
		fileFormat:     fileFormatBase64,
		defaults: webUIDefaults{
			ImageModel:   "test-model",
			ImageSize:    "1024x1024",
			ImageQuality: "auto",
			ImageCount:   1,
		},
		saveDir: saveDir,
	}

	body := &strings.Builder{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("prompt", "hello")
	_ = writer.WriteField("image_model", "test-model")
	_ = writer.WriteField("image_endpoint", "response")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	ui.handleGenerate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	if len(gotRequest.Input) < 2 {
		t.Fatalf("unexpected input messages: %#v", gotRequest.Input)
	}
	if gotRequest.Input[0].Role != "system" {
		t.Fatalf("expected system role, got: %q", gotRequest.Input[0].Role)
	}
	if len(gotRequest.Input[0].Content) != 1 || gotRequest.Input[0].Content[0].Text != "be concise" {
		t.Fatalf("unexpected system content: %#v", gotRequest.Input[0].Content)
	}
}
