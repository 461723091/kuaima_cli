//go:build !windows

package app

type webUIRuntimeOptions struct {
	HideConsole bool
}

func hideWebUIConsole() {}

func startWebUITray(string, func()) func() {
	return nil
}
