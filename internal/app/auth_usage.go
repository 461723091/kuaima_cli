package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *client) getTokenUsage(ctx context.Context) (*tokenUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/usage/token/2", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	c.logRequest(req, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.logResponse(resp, data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("usage request failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var payload tokenUsageResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode usage response: %w", err)
	}
	if !payload.Code {
		if strings.TrimSpace(payload.Message) == "" {
			payload.Message = "usage request failed"
		}
		return nil, errors.New(payload.Message)
	}
	return &payload.Data, nil
}
