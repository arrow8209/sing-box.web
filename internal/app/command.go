package app

import (
  "context"
  "fmt"
  "io"
  "slices"
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

func runCommand(_ context.Context, command Command, _ io.Reader, _ io.Writer, stderr io.Writer) int {
  fmt.Fprintf(stderr, "命令 %s 尚未接通对应服务组件\n", command)
  return 1
}
