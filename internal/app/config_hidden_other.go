//go:build !windows

package app

func hideAppConfigDir(string) error {
	return nil
}
