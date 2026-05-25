//go:build windows

package app

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestLoadAppConfigHidesExecutableConfigDirOnWindows(t *testing.T) {
	exeDir := t.TempDir()
	homeDir := t.TempDir()
	resetConfigPathFuncs(t, filepath.Join(exeDir, "kuaima_cli.exe"), homeDir)
	t.Setenv("KUAIMA_CONFIG_DIR", "")

	if _, err := loadAppConfig(); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(exeDir, configDirName)
	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := syscall.GetFileAttributes(path)
	if err != nil {
		t.Fatal(err)
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		t.Fatalf("expected %s to be hidden", dir)
	}
}
