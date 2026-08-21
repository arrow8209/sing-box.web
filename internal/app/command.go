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
	CommandServe       Command = "serve"
	CommandSetPassword Command = "admin-set-password"
)

func ParseCommand(args []string) (Command, error) {
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

func runCommand(_ context.Context, command Command, stdin io.Reader, stdout, stderr io.Writer) int {
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
	fmt.Fprintf(stderr, "命令 %s 尚未接通对应服务组件\n", command)
	return 1
}
