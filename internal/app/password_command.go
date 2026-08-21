package app

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

var errPasswordsDoNotMatch = errors.New("两次输入的密码不一致")

type PasswordPrompter interface {
	ReadPassword(prompt string) ([]byte, error)
}

type terminalPasswordPrompter struct {
	input  *os.File
	output io.Writer
}

func (p terminalPasswordPrompter) ReadPassword(prompt string) ([]byte, error) {
	if !term.IsTerminal(int(p.input.Fd())) {
		return nil, errors.New("设置管理密码需要交互式终端")
	}
	if _, err := fmt.Fprint(p.output, prompt); err != nil {
		return nil, err
	}
	password, err := term.ReadPassword(int(p.input.Fd()))
	if _, writeErr := fmt.Fprintln(p.output); writeErr != nil && err == nil {
		return nil, writeErr
	}
	return password, err
}

func runSetPassword(layout paths.Layout, prompt PasswordPrompter, stdout io.Writer) error {
	lock, err := storage.AcquireInstanceLock(layout.AppLock)
	if err != nil {
		return fmt.Errorf("无法在 Web 服务运行时通过 CLI 修改密码: %w", err)
	}
	defer lock.Close()
	first, err := prompt.ReadPassword("请输入新密码：")
	if err != nil {
		return err
	}
	defer clear(first)
	second, err := prompt.ReadPassword("请再次输入新密码：")
	if err != nil {
		return err
	}
	defer clear(second)
	if subtle.ConstantTimeCompare(first, second) != 1 {
		return errPasswordsDoNotMatch
	}
	if err := auth.NewStore(layout.AuthFile, rand.Reader).SetPassword(first); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "管理密码已设置，全部既有会话已失效。")
	return err
}

func clear(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
