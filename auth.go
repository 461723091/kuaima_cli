package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type userLoginResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

type tokenUsage struct {
	Object         string              `json:"object"`
	Name           string              `json:"name"`
	Subscriptions  []tokenSubscription `json:"subscriptions"`
	TotalGranted   int64               `json:"total_granted"`
	TotalUsed      int64               `json:"total_used"`
	TotalAvailable int64               `json:"total_available"`
	UnlimitedQuota bool                `json:"unlimited_quota"`
	ExpiresAt      int64               `json:"expires_at"`
}

type tokenSubscription struct {
	Subscription subscriptionPlan `json:"subscription"`
}

type subscriptionPlan struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"user_id"`
	PlanID        int64  `json:"plan_id"`
	AmountTotal   int64  `json:"amount_total"`
	AmountUsed    int64  `json:"amount_used"`
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	Status        string `json:"status"`
	Source        string `json:"source"`
	LastResetTime int64  `json:"last_reset_time"`
	NextResetTime int64  `json:"next_reset_time"`
	UpgradeGroup  string `json:"upgrade_group"`
	PrevUserGroup string `json:"prev_user_group"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type tokenUsageResponse struct {
	Code    bool       `json:"code"`
	Message string     `json:"message"`
	Data    tokenUsage `json:"data"`
}

func ensureAPIKey(ctx context.Context, baseURL, username, password string, verbose bool, logWriter io.Writer) (string, error) {
	cfg, err := loadAppConfig()
	if err != nil {
		return "", err
	}
	username, password, _, err = resolveStoredCredentials(cfg, username, password)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		username, password, err = generateCredentials()
		if err != nil {
			return "", err
		}
	}

	token, err := loginUser(ctx, baseURL, username, password, verbose, logWriter)
	if err != nil {
		return "", err
	}
	cfg.Username = stringPtr(username)
	cfg.Password = stringPtr(password)
	cfg.APIKey = stringPtr(token)
	if err := saveAppConfig(cfg); err != nil {
		return "", err
	}
	return token, nil
}

func resolveRechargeCredentials(ctx context.Context, baseURL, username, password string, verbose bool, logWriter io.Writer) (string, string, error) {
	cfg, err := loadAppConfig()
	if err != nil {
		return "", "", err
	}
	username, password, changed, err := resolveStoredCredentials(cfg, username, password)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		username, password, err = generateCredentials()
		if err != nil {
			return "", "", err
		}
		changed = true
	}
	if strings.TrimSpace(configString(cfg.APIKey, "")) == "" && strings.TrimSpace(envOr("KUAIMA_API_KEY", os.Getenv("OPENAI_API_KEY"))) == "" {
		token, err := loginUser(ctx, baseURL, username, password, verbose, logWriter)
		if err != nil {
			return "", "", err
		}
		cfg.APIKey = stringPtr(token)
		changed = true
	}
	if configString(cfg.Username, "") != username {
		cfg.Username = stringPtr(username)
		changed = true
	}
	if configString(cfg.Password, "") != password {
		cfg.Password = stringPtr(password)
		changed = true
	}
	if changed {
		if err := saveAppConfig(cfg); err != nil {
			return "", "", err
		}
	}
	return username, password, nil
}

func resolveStoredCredentials(cfg appConfig, username, password string) (string, string, bool, error) {
	changed := false
	if strings.TrimSpace(username) == "" {
		username = configString(cfg.Username, "")
	}
	if strings.TrimSpace(password) == "" {
		password = configString(cfg.Password, "")
	}
	if strings.TrimSpace(username) != "" && strings.TrimSpace(password) != "" {
		return username, password, changed, nil
	}

	legacyUsername, legacyPassword, ok, err := readLegacyBuildCredentials()
	if err != nil {
		return "", "", false, err
	}
	if ok {
		if strings.TrimSpace(username) == "" {
			username = legacyUsername
			changed = true
		}
		if strings.TrimSpace(password) == "" {
			password = legacyPassword
			changed = true
		}
	}
	return username, password, changed, nil
}

func loginUser(ctx context.Context, baseURL, username, password string, verbose bool, logWriter io.Writer) (string, error) {
	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/user/ulogin"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if logEnabled(verbose, logWriter) {
		w := logOutput(verbose, logWriter)
		fmt.Fprintf(w, "\n%s --- request ---\nPOST %s\n", logEntryPrefix(2), endpoint)
		fmt.Fprintln(w, "Content-Type: application/json")
		fmt.Fprintln(w, "Accept: application/json")
		fmt.Fprintf(w, "{\n  \"username\": %q,\n  \"password\": \"<redacted>\"\n}\n", username)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if logEnabled(verbose, logWriter) {
		w := logOutput(verbose, logWriter)
		fmt.Fprintf(w, "\n%s --- response ---\n%s\n", logEntryPrefix(2), resp.Status)
		fmt.Fprintln(w, prettyJSON(data))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login failed: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var payload userLoginResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if !payload.Success {
		if strings.TrimSpace(payload.Message) == "" {
			payload.Message = "unknown login error"
		}
		return "", errors.New(payload.Message)
	}
	if strings.TrimSpace(payload.Data.Token) == "" {
		return "", errors.New("login response did not include token")
	}
	return payload.Data.Token, nil
}

func readLegacyBuildCredentials() (string, string, bool, error) {
	path := filepath.Clean("../../../../config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	var legacy struct {
		BuildInfo struct {
			Username string `json:"u_info"`
			Password string `json:"key_info"`
		} `json:"build_info"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return "", "", false, fmt.Errorf("read legacy config %s: %w", path, err)
	}
	username := strings.TrimSpace(legacy.BuildInfo.Username)
	password := strings.TrimSpace(legacy.BuildInfo.Password)
	if username == "" || password == "" {
		return "", "", false, nil
	}
	return username, password, true, nil
}

func MD5(str string) string {
	sum := md5.Sum([]byte(str))
	return hex.EncodeToString(sum[:])
}

// 生成账号密码
func generateCredentials() (string, string, error) {
	userSuffix, err := randomHex(6)
	if err != nil {
		return "", "", err
	}
	passSuffix, err := randomHex(12)
	if err != nil {
		return "", "", err
	}

	userSuffix = MD5(userSuffix+time.Now().Format(time.RFC3339Nano)) + "_cli"

	return userSuffix, MD5(passSuffix), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func rechargeURL(baseURL, username, password string, now time.Time) (string, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/user/ulogin2")
	if err != nil {
		return "", err
	}
	password = MD5(fmt.Sprintf("%s%d%s", password, now.Unix(), username))
	query := endpoint.Query()
	query.Set("username", username)
	query.Set("password", password)
	query.Set("create_time", fmt.Sprintf("%d", now.Unix()))
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

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
