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

	defaultOssURL = "https://oss.szkmjb.com"
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
		//case "login":
		//	exit(runLogin(os.Args[2:]))
		//	return
		case "balance":
			exit(runBalance(os.Args[2:]))
			return
		case "recharge":
			exit(runRecharge(os.Args[2:]))
			return
		case "help", "-h", "--help":
			usage(os.Stdout)
			return
		}
	}

	if len(os.Args) > 1 {
		exit(runAsk(os.Args[1:]))
		return
	}
	usage(os.Stdout)
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
