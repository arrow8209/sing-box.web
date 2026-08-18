package main

import (
	"context"
	"os"

	"singbox-web/internal/cli"
)

var version = "dev"

func main() {
	deps := cli.Dependencies{Version: version}
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, deps))
}
