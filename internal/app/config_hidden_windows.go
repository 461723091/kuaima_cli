//go:build windows

package app

import "syscall"

func hideAppConfigDir(dir string) error {
	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(path)
	if err != nil {
		return err
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return nil
	}
	return syscall.SetFileAttributes(path, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
