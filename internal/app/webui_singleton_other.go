//go:build !windows

package app

import (
	"fmt"
	"net"
	"strings"
)

func acquireWebUIInstance() (func(), bool, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:8789")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "address already in use") {
			return func() {}, false, nil
		}
		return nil, false, err
	}
	return func() { _ = ln.Close() }, true, nil
}

func notifyWebUIAlreadyRunning(message string) {
	fmt.Println(message)
}
