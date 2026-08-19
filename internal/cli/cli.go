package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

type ServeOptions struct {
	Listen            string
	DataDir           string
	ConfigRoot        string
	SingBoxPath       string
	ConfigGroup       string
	AllowPublicListen bool
	Development       bool
	PublicOrigin      string
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
	if len(args) >= 1 && args[0] == "serve" {
		return runServe(ctx, args[1:], stderr, deps)
	}
	fmt.Fprintln(stderr, "用法: singbox-web <version|admin|serve>")
	return 2
}

func runServe(ctx context.Context, args []string, stderr io.Writer, deps Dependencies) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := ServeOptions{}
	flags.StringVar(&options.Listen, "listen", "127.0.0.1:8080", "监听地址")
	flags.StringVar(&options.DataDir, "data-dir", "", "数据目录")
	flags.StringVar(&options.ConfigRoot, "config-root", "", "正式配置根目录")
	flags.StringVar(&options.SingBoxPath, "sing-box", "/usr/local/bin/sing-box", "sing-box 路径")
	flags.StringVar(&options.ConfigGroup, "config-group", "sing-box", "正式配置组")
	flags.StringVar(&options.PublicOrigin, "public-origin", "", "外部 HTTPS Origin")
	flags.BoolVar(&options.Development, "dev", false, "开发模式")
	flags.BoolVar(&options.AllowPublicListen, "allow-public-listen", false, "允许公开监听（危险）")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return 2
	}
	if options.Development {
		if options.DataDir == "" {
			options.DataDir = filepath.Join(".singbox-web", "data")
		}
		if options.ConfigRoot == "" {
			options.ConfigRoot = filepath.Join(".singbox-web", "config")
		}
		var err error
		if options.DataDir, err = filepath.Abs(options.DataDir); err != nil {
			fmt.Fprintln(stderr, "无法解析数据目录")
			return 2
		}
		if options.ConfigRoot, err = filepath.Abs(options.ConfigRoot); err != nil {
			fmt.Fprintln(stderr, "无法解析配置目录")
			return 2
		}
		if options.PublicOrigin == "" {
			options.PublicOrigin = "http://" + options.Listen
		}
	} else if options.DataDir == "" || options.ConfigRoot == "" || options.PublicOrigin == "" {
		fmt.Fprintln(stderr, "生产模式必须指定 --data-dir、--config-root 和 --public-origin")
		return 2
	}
	if deps.Serve == nil {
		fmt.Fprintln(stderr, "服务启动不可用")
		return 1
	}
	if err := deps.Serve(ctx, options); err != nil {
		fmt.Fprintln(stderr, "服务启动失败")
		return 1
	}
	return 0
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
	defer clear(password)
	if err != nil {
		fmt.Fprintln(stderr, "读取管理员密码失败")
		return 1
	}
	fmt.Fprint(stderr, "请再次输入管理员密码: ")
	confirmation, err := deps.ReadPassword()
	defer clear(confirmation)
	if err != nil {
		fmt.Fprintln(stderr, "读取管理员密码失败")
		return 1
	}
	if !bytes.Equal(password, confirmation) {
		fmt.Fprintln(stderr, "两次输入的管理员密码不一致")
		return 1
	}
	err = deps.SetPassword(ctx, *dataDir, password)
	if err != nil {
		fmt.Fprintln(stderr, "设置管理员密码失败")
		return 1
	}
	fmt.Fprintln(stderr, "管理员密码已设置")
	return 0
}
