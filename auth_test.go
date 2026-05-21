package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureAPIKeyLogsInAndSavesCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUAIMA_CONFIG_DIR", dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/ulogin" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["username"] != "user1" || req["password"] != "pass1" {
			t.Fatalf("unexpected login request: %#v", req)
		}
		_, _ = w.Write([]byte(`{"message":"","success":true,"data":{"token":"token1"}}`))
	}))
	defer server.Close()

	token, err := ensureAPIKey(t.Context(), server.URL, "user1", "pass1", false)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token1" {
		t.Fatalf("unexpected token: %q", token)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if configString(cfg.Username, "") != "user1" || configString(cfg.Password, "") != "pass1" || configString(cfg.APIKey, "") != "token1" {
		t.Fatalf("unexpected saved config: %#v", cfg)
	}
}

func TestRechargeURL(t *testing.T) {
	got, err := rechargeURL("https://ai.example.com/", "u+1", "p&1", time.Unix(123, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := "https://ai.example.com/api/user/ulogin2?create_time=123&password=p%261&username=u%2B1"
	if got != want {
		t.Fatalf("unexpected recharge URL:\nwant %s\n got %s", want, got)
	}
}

func TestGetTokenUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/usage/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token1" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"code":true,"message":"ok","data":{"object":"token_usage","name":"Default Token","total_granted":1000000,"total_used":12345,"total_available":987655,"unlimited_quota":false,"expires_at":0}}`))
	}))
	defer server.Close()

	c := &client{baseURL: server.URL, apiKey: "token1", httpClient: server.Client()}
	usage, err := c.getTokenUsage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if usage.TotalAvailable != 987655 || usage.TotalUsed != 12345 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}
