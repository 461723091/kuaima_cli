package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"kuaima_cli/internal/app"
)

func main() {
	exit(app.Run(os.Args[1:]))
}

func exit(err error) {
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fatal(err)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
