package cli

import (
	"bytes"
	"context"
	"flag"
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

func Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer, deps Dependencies) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "singbox-web %s\n", deps.Version)
		return 0
	}
	if len(args) >= 2 && args[0] == "admin" && args[1] == "set-password" {
		return runAdminSetPassword(ctx, args[2:], stderr, deps)
	}
	fmt.Fprintln(stderr, "用法: singbox-web <version|admin|serve>")
	return 2
}

func runAdminSetPassword(ctx context.Context, args []string, stderr io.Writer, deps Dependencies) int {
	flags := flag.NewFlagSet("admin set-password", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "数据目录")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *dataDir == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "用法: singbox-web admin set-password --data-dir <目录>")
		return 2
	}
	if deps.ReadPassword == nil || deps.SetPassword == nil {
		fmt.Fprintln(stderr, "管理员密码初始化不可用")
		return 1
	}

	fmt.Fprint(stderr, "请输入管理员密码: ")
	password, err := deps.ReadPassword()
	if err != nil {
		fmt.Fprintln(stderr, "读取管理员密码失败")
		return 1
	}
	fmt.Fprint(stderr, "请再次输入管理员密码: ")
	confirmation, err := deps.ReadPassword()
	if err != nil {
		clear(password)
		fmt.Fprintln(stderr, "读取管理员密码失败")
		return 1
	}
	if !bytes.Equal(password, confirmation) {
		clear(password)
		clear(confirmation)
		fmt.Fprintln(stderr, "两次输入的管理员密码不一致")
		return 1
	}
	err = deps.SetPassword(ctx, *dataDir, password)
	clear(password)
	clear(confirmation)
	if err != nil {
		fmt.Fprintln(stderr, "设置管理员密码失败")
		return 1
	}
	fmt.Fprintln(stderr, "管理员密码已设置")
	return 0
}
