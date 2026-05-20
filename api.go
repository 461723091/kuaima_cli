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
	"strings"
	"time"
)

type client struct {
	baseURL    string
	ossURL     string
	apiKey     string
	verbose    bool
	httpClient *http.Client
}

type responseRequest struct {
	Model  string `json:"model"`
	Input  any    `json:"input"`
	Tools  any    `json:"tools,omitempty"`
	Stream bool   `json:"stream,omitempty"`
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

func newClient(baseURL, ossURL, apiKey string, verbose bool) (*client, error) {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = envOr("KUAIMA_API_KEY", os.Getenv("OPENAI_API_KEY"))
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("missing API key: pass -api-key or set KUAIMA_API_KEY or OPENAI_API_KEY")
	}
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		ossURL:  strings.TrimRight(ossURL, "/"),
		apiKey:  apiKey,
		verbose: verbose,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}, nil
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
	c.logRequest(httpReq, body)
	httpReq.Header.Set("Accept", "application/json")

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
	c.logRequest(httpReq, body)
	httpReq.Header.Set("Accept", "text/event-stream")

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
	if !c.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "\n--- request ---\n%s %s\n", req.Method, req.URL.String())
	fmt.Fprintln(os.Stderr, "Authorization: Bearer <redacted>")
	fmt.Fprintln(os.Stderr, "Content-Type: application/json")
	fmt.Fprintln(os.Stderr, prettyJSON(body))
}

func (c *client) logTiming(started, first time.Time) {
	if !c.verbose {
		return
	}
	fmt.Fprintln(os.Stderr, "\n--- timing ---")
	if first.IsZero() {
		fmt.Fprintln(os.Stderr, "首字时间: n/a")
	} else {
		fmt.Fprintf(os.Stderr, "首字时间: %s\n", first.Sub(started).Round(time.Millisecond))
	}
	fmt.Fprintf(os.Stderr, "完整时间: %s\n", time.Since(started).Round(time.Millisecond))
}

func (c *client) logResponse(resp *http.Response, body []byte) {
	if !c.verbose {
		return
	}
	c.logResponseStatus(resp)
	c.logResponseBody(body)
}

func (c *client) logResponseStatus(resp *http.Response) {
	if !c.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "\n--- response ---\n%s\n", resp.Status)
}

func (c *client) logResponseBody(body []byte) {
	if !c.verbose {
		return
	}
	if len(body) == 0 {
		fmt.Fprintln(os.Stderr, "<empty>")
		return
	}
	fmt.Fprintln(os.Stderr, prettyJSON(body))
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
