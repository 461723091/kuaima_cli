//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

const (
	errorAlreadyExists = 183
)

var (
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
	procReleaseMutex = kernel32.NewProc("ReleaseMutex")
	procCloseHandle  = kernel32.NewProc("CloseHandle")
	user32Singleton  = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW  = user32Singleton.NewProc("MessageBoxW")
)

func acquireWebUIInstance() (func(), bool, error) {
	name := syscall.StringToUTF16Ptr(`Local\KuaimaCLIWebUI`)
	handle, _, callErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		if callErr != syscall.Errno(0) {
			return nil, false, callErr
		}
		return nil, false, syscall.EINVAL
	}
	if callErr == syscall.Errno(errorAlreadyExists) {
		procCloseHandle.Call(handle)
		return func() {}, false, nil
	}
	return func() {
		procReleaseMutex.Call(handle)
		procCloseHandle.Call(handle)
	}, true, nil
}

func notifyWebUIAlreadyRunning(message string) {
	title := syscall.StringToUTF16Ptr("Kuaima AI WebUI")
	text := syscall.StringToUTF16Ptr(message)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0)
}
