package app

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

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

func openFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return openBrowser(abs)
}
