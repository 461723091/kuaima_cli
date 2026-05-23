package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type client struct {
	baseURL    string
	ossURL     string
	apiKey     string
	verbose    bool
	logWriter  io.Writer
	logCloser  io.Closer
	httpClient *http.Client
}

type responseRequest struct {
	Model  string `json:"model"`
	Input  any    `json:"input"`
	Tools  any    `json:"tools,omitempty"`
	Stream bool   `json:"stream,omitempty"`
}

type imageGenerationRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	N                 int    `json:"n,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	Background        string `json:"background,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
}

type imageEditRequest struct {
	Model             string     `json:"model"`
	Prompt            string     `json:"prompt"`
	Images            []imageRef `json:"images"`
	Mask              *imageRef  `json:"mask,omitempty"`
	N                 int        `json:"n,omitempty"`
	Size              string     `json:"size,omitempty"`
	Quality           string     `json:"quality,omitempty"`
	OutputFormat      string     `json:"output_format,omitempty"`
	OutputCompression *int       `json:"output_compression,omitempty"`
	Background        string     `json:"background,omitempty"`
	Moderation        string     `json:"moderation,omitempty"`
}

type imageRef struct {
	FileID   string `json:"file_id,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type responsePayload struct {
	ID         string           `json:"id"`
	OutputText string           `json:"output_text"`
	Output     []responseOutput `json:"output"`
	Error      *apiError        `json:"error"`
	Raw        json.RawMessage  `json:"-"`
}

type responseOutput struct {
	Type    string            `json:"type"`
	Role    string            `json:"role"`
	Content []responseContent `json:"content"`
	Result  string            `json:"result"`
}

type responseContent struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func newClient(baseURL, ossURL, apiKey, username, password string, verbose bool, logPath string) (*client, error) {
	logFile, err := openLogFile(logPath)
	if err != nil {
		return nil, err
	}
	var logWriter io.Writer
	var logCloser io.Closer
	if logFile != nil {
		logWriter = logFile
		logCloser = logFile
	}

	if strings.TrimSpace(apiKey) == "" {
		apiKey = envOr("KUAIMA_API_KEY", os.Getenv("OPENAI_API_KEY"))
	}
	if strings.TrimSpace(apiKey) == "" {
		var err error
		apiKey, err = ensureAPIKey(context.Background(), baseURL, username, password, verbose, logWriter)
		if err != nil {
			if logCloser != nil {
				_ = logCloser.Close()
			}
			return nil, fmt.Errorf("auto get api-key: %w", err)
		}
	}
	return &client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		ossURL:    strings.TrimRight(ossURL, "/"),
		apiKey:    apiKey,
		verbose:   verbose,
		logWriter: logWriter,
		logCloser: logCloser,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}, nil
}

func openLogFile(path string) (*os.File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}
	return file, nil
}

func (c *client) Close() error {
	if c == nil || c.logCloser == nil {
		return nil
	}
	return c.logCloser.Close()
}

func (c *client) createResponse(ctx context.Context, req responseRequest) (*responsePayload, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	c.logRequest(httpReq, body)

	started := time.Now()
	var firstByte time.Time
	httpReq = withFirstByteTrace(httpReq, &firstByte)
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	c.logResponse(httpResp, data)
	c.logTiming(started, firstByte)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed: %s: %s", httpResp.Status, strings.TrimSpace(string(data)))
	}

	return decodeResponse(data)
}

func (c *client) createResponseStream(ctx context.Context, req responseRequest, w io.Writer) (*responsePayload, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := c.newRequest(ctx, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	c.logRequest(httpReq, body)

	started := time.Now()
	var firstByte time.Time
	httpReq = withFirstByteTrace(httpReq, &firstByte)
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	c.logResponseStatus(httpResp)

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		data, _ := io.ReadAll(httpResp.Body)
		c.logResponseBody(data)
		c.logTiming(started, firstByte)
		return nil, fmt.Errorf("API request failed: %s: %s", httpResp.Status, strings.TrimSpace(string(data)))
	}

	var completed json.RawMessage
	var text bytes.Buffer
	var rawStream bytes.Buffer
	var firstToken time.Time
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		rawStream.WriteString(line)
		rawStream.WriteByte('\n')
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Text     string          `json:"text"`
			Response json.RawMessage `json:"response"`
			Error    *apiError       `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Error != nil {
			return nil, fmt.Errorf("API error: %s", event.Error.Message)
		}
		switch event.Type {
		case "response.output_text.delta", "response.refusal.delta":
			if event.Delta != "" {
				if firstToken.IsZero() {
					firstToken = time.Now()
				}
				fmt.Fprint(w, event.Delta)
				text.WriteString(event.Delta)
			}
		case "response.completed":
			completed = event.Response
		case "response.failed":
			if event.Response != nil {
				resp, err := decodeResponse(event.Response)
				if err == nil && resp.Error != nil {
					return nil, fmt.Errorf("API error: %s", resp.Error.Message)
				}
			}
			return nil, errors.New("API stream failed")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	c.logResponseBody(rawStream.Bytes())
	if firstToken.IsZero() {
		firstToken = firstByte
	}
	c.logTiming(started, firstToken)
	if len(completed) > 0 {
		return decodeResponse(completed)
	}
	return &responsePayload{OutputText: text.String(), Raw: []byte(`{}`)}, nil
}

func (c *client) newRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/responses", body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func withFirstByteTrace(req *http.Request, firstByte *time.Time) *http.Request {
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			if firstByte.IsZero() {
				*firstByte = time.Now()
			}
		},
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (c *client) logRequest(req *http.Request, body []byte) {
	if !logEnabled(c.verbose, c.logWriter) {
		return
	}
	w := logOutput(c.verbose, c.logWriter)
	fmt.Fprintf(w, "\n%s --- request ---\n%s %s\n", logEntryPrefix(3), req.Method, req.URL.String())
	if req.Header.Get("Authorization") != "" {
		fmt.Fprintln(w, "Authorization: Bearer <redacted>")
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "" {
		fmt.Fprintf(w, "Content-Type: %s\n", contentType)
	}
	if accept := req.Header.Get("Accept"); accept != "" {
		fmt.Fprintf(w, "Accept: %s\n", accept)
	}
	if len(body) > 0 {
		fmt.Fprintln(w, prettyJSON(body))
	}
}

func (c *client) logTiming(started, first time.Time) {
	if !logEnabled(c.verbose, c.logWriter) {
		return
	}
	w := logOutput(c.verbose, c.logWriter)
	fmt.Fprintf(w, "\n%s --- timing ---\n", logEntryPrefix(3))
	if first.IsZero() {
		fmt.Fprintln(w, "首字时间: n/a")
	} else {
		fmt.Fprintf(w, "首字时间: %s\n", first.Sub(started).Round(time.Millisecond))
	}
	fmt.Fprintf(w, "完整时间: %s\n", time.Since(started).Round(time.Millisecond))
}

func (c *client) logResponse(resp *http.Response, body []byte) {
	if !logEnabled(c.verbose, c.logWriter) {
		return
	}
	c.logResponseStatusWithCaller(resp, logCaller(2))
	c.logResponseBody(body)
}

func (c *client) logResponseStatus(resp *http.Response) {
	c.logResponseStatusWithCaller(resp, logCaller(2))
}

func (c *client) logResponseStatusWithCaller(resp *http.Response, caller string) {
	if !logEnabled(c.verbose, c.logWriter) {
		return
	}
	fmt.Fprintf(logOutput(c.verbose, c.logWriter), "\n%s --- response ---\n%s\n", logEntryPrefixForCaller(caller), resp.Status)
}

func (c *client) logResponseBody(body []byte) {
	if !logEnabled(c.verbose, c.logWriter) {
		return
	}
	w := logOutput(c.verbose, c.logWriter)
	if len(body) == 0 {
		fmt.Fprintln(w, "<empty>")
		return
	}
	fmt.Fprintln(w, prettyJSON(body))
}

func logEnabled(verbose bool, writer io.Writer) bool {
	return verbose || writer != nil
}

func logOutput(verbose bool, writer io.Writer) io.Writer {
	if verbose && writer != nil {
		return io.MultiWriter(os.Stderr, writer)
	}
	if writer != nil {
		return writer
	}
	if verbose {
		return os.Stderr
	}
	return io.Discard
}

func logEntryPrefix(skip int) string {
	return logEntryPrefixForCaller(logCaller(skip))
}

func logEntryPrefixForCaller(caller string) string {
	return fmt.Sprintf("[%s] [%s]", time.Now().Format("2006-01-02 15:04:05.000"), caller)
}

func logCaller(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	name := fn.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func prettyJSON(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return string(data)
	}
	formatted, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(formatted)
}

func decodeResponse(data []byte) (*responsePayload, error) {
	var payload responsePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	payload.Raw = append(payload.Raw[:0], data...)
	if payload.Error != nil {
		return nil, fmt.Errorf("API error: %s", payload.Error.Message)
	}
	return &payload, nil
}

func (c *client) createImageGeneration(ctx context.Context, req imageGenerationRequest) (*responsePayload, error) {
	return c.createImageRequest(ctx, "/v1/images/generations", req, "image generation")
}

func (c *client) createImageEdit(ctx context.Context, req imageEditRequest) (*responsePayload, error) {
	return c.createImageRequest(ctx, "/v1/images/edits", req, "image edit")
}

func (c *client) createImageRequest(ctx context.Context, path string, req any, operation string) (*responsePayload, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	c.logRequest(httpReq, body)

	started := time.Now()
	var firstByte time.Time
	httpResp, err := c.httpClient.Do(withFirstByteTrace(httpReq, &firstByte))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	c.logResponse(httpResp, data)
	c.logTiming(started, firstByte)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s failed: %s: %s", operation, httpResp.Status, strings.TrimSpace(string(data)))
	}
	return decodeResponse(data)
}

func (r *responsePayload) text() string {
	if strings.TrimSpace(r.OutputText) != "" {
		return r.OutputText
	}
	var parts []string
	for _, out := range r.Output {
		for _, content := range out.Content {
			if content.Text != "" {
				parts = append(parts, content.Text)
			}
			if content.ImageURL != "" {
				parts = append(parts, content.ImageURL)
			}
		}
	}
	return strings.Join(parts, "\n")
}
