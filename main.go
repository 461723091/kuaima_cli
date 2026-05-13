package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

const (
	defaultBaseURL = "https://ai.szkmjb.com"
	defaultModel   = "gpt-5.4-mini"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "ask":
			exit(runAsk(os.Args[2:]))
			return
		case "chat":
			exit(runChat(os.Args[2:]))
			return
		case "image":
			exit(runImage(os.Args[2:]))
			return
		case "help", "-h", "--help":
			usage(os.Stdout)
			return
		}
	}

	exit(runAsk(os.Args[1:]))
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
