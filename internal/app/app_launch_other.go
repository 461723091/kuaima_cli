//go:build !windows && !darwin

package app

func launchedFromFileExplorer() bool {
	return false
}
