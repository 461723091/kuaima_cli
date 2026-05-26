//go:build darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func launchedFromFileExplorer() bool {
	parent, err := darwinProcessName(os.Getppid())
	if err != nil {
		return false
	}
	switch strings.ToLower(parent) {
	case "finder", "launchd", "open":
		return true
	default:
		return false
	}
}

func darwinProcessName(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", nil
	}
	return filepath.Base(name), nil
}
