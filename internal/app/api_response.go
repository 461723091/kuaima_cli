package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

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
		return nil, apiRequestError("API request", httpResp.Status, data)
	}

	return decodeResponse(data)
}

func decodeResponse(data []byte) (*responsePayload, error) {
	var payload responsePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	payload.Raw = append(payload.Raw[:0], data...)
	if payload.Error != nil {
		return nil, apiErrorf("API error: %s", apiErrorMessage(payload.Error))
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
