package cli

import (
	"context"
	"fmt"
	"io"
)

type ServeOptions struct {
	Listen      string
	DataDir     string
	ConfigRoot  string
	SingBoxPath string
	ConfigGroup string
}

type Dependencies struct {
	Version      string
	ReadPassword func() ([]byte, error)
	SetPassword  func(context.Context, string, []byte) error
	Serve        func(context.Context, ServeOptions) error
}

func Run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer, deps Dependencies) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "singbox-web %s\n", deps.Version)
		return 0
	}
	fmt.Fprintln(stderr, "用法: singbox-web <version|admin|serve>")
	return 2
}
