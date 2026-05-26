//go:build windows

package app

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	th32csSnapProcess = 0x00000002
	maxPath           = 260
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelpSnapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW        = kernel32.NewProc("Process32FirstW")
	procProcess32NextW         = kernel32.NewProc("Process32NextW")
)

type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PcPriClassBase  int32
	Flags           uint32
	ExeFile         [maxPath]uint16
}

func launchedFromFileExplorer() bool {
	parent, err := parentProcessName(uint32(os.Getppid()))
	return err == nil && strings.EqualFold(parent, "explorer.exe")
}

func parentProcessName(pid uint32) (string, error) {
	handle, _, err := procCreateToolhelpSnapshot.Call(th32csSnapProcess, 0)
	if handle == uintptr(syscall.InvalidHandle) {
		return "", err
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	entry := processEntry32W{Size: uint32(unsafe.Sizeof(processEntry32W{}))}
	ok, _, err := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		if entry.ProcessID == pid {
			return syscall.UTF16ToString(entry.ExeFile[:]), nil
		}
		ok, _, err = procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&entry)))
	}
	return "", err
}
