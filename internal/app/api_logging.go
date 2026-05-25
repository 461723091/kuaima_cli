package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

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
