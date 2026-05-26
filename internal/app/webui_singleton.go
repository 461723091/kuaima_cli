package app

import (
	"os"
	"path/filepath"
	"strings"
)

const webUIInstanceFileName = "webui-instance.txt"

func webUIInstanceURLPath() string {
	if dir := strings.TrimSpace(os.Getenv("KUAIMA_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, webUIInstanceFileName)
	}
	if path, err := appConfigPath(); err == nil {
		return filepath.Join(filepath.Dir(path), webUIInstanceFileName)
	}
	return filepath.Join(os.TempDir(), "kuaima-"+webUIInstanceFileName)
}

func readWebUIInstanceURL() string {
	data, err := os.ReadFile(webUIInstanceURLPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeWebUIInstanceURL(url string) error {
	path := webUIInstanceURLPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(url)+"\n"), 0600)
}

func removeWebUIInstanceURL() {
	_ = os.Remove(webUIInstanceURLPath())
}
