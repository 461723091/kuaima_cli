package main

import (
	"net/url"
	"os"
	"strings"
)

func cleanBase64(value string) string {
	replacer := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "")
	return replacer.Replace(value)
}

func isHTTPURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
