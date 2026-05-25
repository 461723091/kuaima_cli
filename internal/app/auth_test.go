package app

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

	token, err := ensureAPIKey(t.Context(), server.URL, "user1", "pass1", false, nil)
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
	want := "https://ai.example.com/api/user/ulogin2?create_time=123&password=486bdaad25325e20aebc9b316a46b8ac&username=u%2B1"
	if got != want {
		t.Fatalf("unexpected recharge URL:\nwant %s\n got %s", want, got)
	}
}

func TestGetTokenUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/usage/token/2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token1" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"code":true,"data":{"expires_at":0,"model_limits":{},"model_limits_enabled":false,"name":"root测试","object":"token_usage","subscriptions":[{"subscription":{"id":1,"user_id":1,"plan_id":1,"amount_total":500000,"amount_used":499255,"start_time":1778486332,"end_time":1781164732,"status":"active","source":"order","last_reset_time":0,"next_reset_time":0,"upgrade_group":"","prev_user_group":"","created_at":1778486331,"updated_at":1778821959}}],"total_available":219586644,"total_granted":651955744,"total_used":432369100,"unlimited_quota":true},"message":"ok"}`))
	}))
	defer server.Close()

	c := &client{baseURL: server.URL, apiKey: "token1", httpClient: server.Client()}
	usage, err := c.getTokenUsage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if usage.TotalAvailable != 219586644 || usage.TotalUsed != 432369100 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if len(usage.Subscriptions) != 1 {
		t.Fatalf("unexpected subscriptions: %#v", usage.Subscriptions)
	}
	if usage.Subscriptions[0].Subscription.AmountTotal != 500000 || usage.Subscriptions[0].Subscription.Status != "active" {
		t.Fatalf("unexpected subscription: %#v", usage.Subscriptions[0].Subscription)
	}
}

func TestFormatQuotaAmount(t *testing.T) {
	if got, want := formatQuotaAmount(500000), "500000 (￥1.00)"; got != want {
		t.Fatalf("unexpected amount:\nwant %s\n got %s", want, got)
	}
}
