package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type Command string

const (
	CommandHelp        Command = "help"
	CommandServe       Command = "serve"
	CommandSetPassword Command = "admin-set-password"
)

const commandHelp = `SingBox Control

用法：singbox-web <命令>

命令：
  serve               启动 Web 管理服务
  admin set-password  交互式设置管理密码

选项：
  -h, --help          显示帮助并退出

数据与配置固定保存在启动工作目录下的 data/ 和 config/。
服务设置位于 data/settings.json，默认 Web 端口为 50022，默认 Mixed 入站端口为 50023，默认 Clash API 端口为 50024。
`

func ParseCommand(args []string) (Command, error) {
	if slices.Equal(args, []string{"-h"}) || slices.Equal(args, []string{"--help"}) {
		return CommandHelp, nil
	}
	if slices.Equal(args, []string{"serve"}) {
		return CommandServe, nil
	}
	if slices.Equal(args, []string{"admin", "set-password"}) {
		return CommandSetPassword, nil
	}
	return "", fmt.Errorf("用法：singbox-web serve | singbox-web admin set-password")
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command, err := ParseCommand(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return runCommand(ctx, command, stdin, stdout, stderr)
}

func runCommand(ctx context.Context, command Command, stdin io.Reader, stdout, stderr io.Writer) int {
	if command == CommandHelp {
		fmt.Fprint(stdout, commandHelp)
		return 0
	}
	if command == CommandSetPassword {
		input, ok := stdin.(*os.File)
		if !ok {
			fmt.Fprintln(stderr, "设置管理密码需要交互式终端")
			return 1
		}
		workDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		layout, err := paths.Resolve(workDir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		err = runSetPassword(layout, terminalPasswordPrompter{input: input, output: stderr}, stdout)
		if err != nil {
			if errors.Is(err, storage.ErrAlreadyLocked) {
				fmt.Fprintln(stderr, "无法在 Web 服务运行时通过 CLI 修改密码")
			} else {
				fmt.Fprintln(stderr, err)
			}
			return 1
		}
		return 0
	}
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	server, err := NewServer(workDir, Dependencies{Output: stdout, EnableServerReferenceResolution: true})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := server.Run(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
