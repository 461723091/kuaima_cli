package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
