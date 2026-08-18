package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run(context.Background(), []string{"version"}, strings.NewReader(""), &stdout, io.Discard, Dependencies{Version: "0.1.0-test"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got, want := stdout.String(), "singbox-web 0.1.0-test\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunAdminSetPassword(t *testing.T) {
	var gotDir string
	var gotPassword []byte
	deps := Dependencies{
		Version:      "test",
		ReadPassword: func() ([]byte, error) { return []byte("a-strong-password"), nil },
		SetPassword: func(_ context.Context, dataDir string, password []byte) error {
			gotDir = dataDir
			gotPassword = append([]byte(nil), password...)
			return nil
		},
	}
	code := Run(context.Background(), []string{"admin", "set-password", "--data-dir", "/var/lib/singbox-web"}, strings.NewReader(""), io.Discard, io.Discard, deps)
	if code != 0 || gotDir != "/var/lib/singbox-web" || string(gotPassword) != "a-strong-password" {
		t.Fatalf("code=%d dir=%q password=%q", code, gotDir, gotPassword)
	}
}

func TestRunAdminSetPasswordZerosPasswords(t *testing.T) {
	password := []byte("a-strong-password")
	confirmation := []byte("a-strong-password")
	readCount := 0
	deps := Dependencies{
		ReadPassword: func() ([]byte, error) {
			readCount++
			if readCount == 1 {
				return password, nil
			}
			return confirmation, nil
		},
		SetPassword: func(_ context.Context, _ string, got []byte) error {
			if string(got) != "a-strong-password" {
				t.Fatalf("SetPassword() password = %q", got)
			}
			return nil
		},
	}
	if code := Run(context.Background(), []string{"admin", "set-password", "--data-dir", "/data"}, strings.NewReader(""), io.Discard, io.Discard, deps); code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	for _, value := range append(password, confirmation...) {
		if value != 0 {
			t.Fatalf("password byte = %d, want 0", value)
		}
	}
}

func TestRunAdminSetPasswordZerosPasswordWhenFirstReadFails(t *testing.T) {
	password := []byte("partially-read-password")
	deps := Dependencies{
		ReadPassword: func() ([]byte, error) {
			return password, errors.New("read failed")
		},
		SetPassword: func(context.Context, string, []byte) error {
			t.Fatal("SetPassword() must not be called")
			return nil
		},
	}
	if code := Run(context.Background(), []string{"admin", "set-password", "--data-dir", "/data"}, strings.NewReader(""), io.Discard, io.Discard, deps); code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	for _, value := range password {
		if value != 0 {
			t.Fatalf("password byte = %d, want 0", value)
		}
	}
}

func TestRunAdminSetPasswordZerosPasswordsWhenSecondReadFails(t *testing.T) {
	password := []byte("a-strong-password")
	confirmation := []byte("partially-read-confirmation")
	readCount := 0
	deps := Dependencies{
		ReadPassword: func() ([]byte, error) {
			readCount++
			if readCount == 1 {
				return password, nil
			}
			return confirmation, errors.New("read failed")
		},
		SetPassword: func(context.Context, string, []byte) error {
			t.Fatal("SetPassword() must not be called")
			return nil
		},
	}
	if code := Run(context.Background(), []string{"admin", "set-password", "--data-dir", "/data"}, strings.NewReader(""), io.Discard, io.Discard, deps); code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	for _, password := range [][]byte{password, confirmation} {
		for _, value := range password {
			if value != 0 {
				t.Fatalf("password byte = %d, want 0", value)
			}
		}
	}
}
