package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

func (c *client) withIndependentHTTPClient() *client {
	if c == nil {
		return nil
	}
	next := *c
	next.logCloser = nil
	next.httpClient = independentHTTPClient(c.httpClient)
	return &next
}

func independentHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: 10 * time.Minute}
	}
	next := &http.Client{
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       base.Timeout,
	}
	switch transport := base.Transport.(type) {
	case *http.Transport:
		cloned := transport.Clone()
		cloned.DisableKeepAlives = true
		next.Transport = cloned
	case nil:
		cloned := http.DefaultTransport.(*http.Transport).Clone()
		cloned.DisableKeepAlives = true
		next.Transport = cloned
	default:
		next.Transport = closeRequestRoundTripper{rt: transport}
	}
	return next
}

type closeRequestRoundTripper struct {
	rt http.RoundTripper
}

func (t closeRequestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	next := req.Clone(req.Context())
	next.Body = req.Body
	next.GetBody = req.GetBody
	next.Close = true
	return t.rt.RoundTrip(next)
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
