package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"singbox-web/internal/app"
	"singbox-web/internal/auth"
	"singbox-web/internal/cli"
)

func dependencies() cli.Dependencies {
	return cli.Dependencies{
		Version:      version,
		ReadPassword: newPasswordReader(os.Stdin),
		SetPassword: func(_ context.Context, dataDir string, password []byte) error {
			return auth.NewFileStore(filepath.Join(dataDir, "auth.json"), time.Now, rand.Reader).SetPassword(password)
		},
		Serve: func(ctx context.Context, options cli.ServeOptions) error {
			return app.Run(ctx, app.Options{
				Listen:            options.Listen,
				DataDir:           options.DataDir,
				ConfigRoot:        options.ConfigRoot,
				SingBoxPath:       options.SingBoxPath,
				ConfigGroup:       options.ConfigGroup,
				AllowPublicListen: options.AllowPublicListen,
				Development:       options.Development,
				PublicOrigin:      options.PublicOrigin,
				Version:           version,
				LogOutput:         os.Stderr,
			})
		},
	}
}

func newPasswordReader(input *os.File) func() ([]byte, error) {
	reader := bufio.NewReader(input)
	return func() ([]byte, error) {
		if term.IsTerminal(int(input.Fd())) {
			return term.ReadPassword(int(input.Fd()))
		}
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if errors.Is(err, io.EOF) && len(line) == 0 {
			return nil, err
		}
		return bytes.TrimRight(line, "\r\n"), nil
	}
}
