package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		args []string
		want Command
	}{
		{[]string{"serve"}, CommandServe},
		{[]string{"admin", "set-password"}, CommandSetPassword},
	}
	for _, test := range tests {
		got, err := ParseCommand(test.args)
		if err != nil || got != test.want {
			t.Fatalf("ParseCommand(%v) = %q, %v", test.args, got, err)
		}
	}
}

func TestParseCommandRejectsPathFlags(t *testing.T) {
	for _, args := range [][]string{
		{"serve", "--data-dir", "/tmp/data"},
		{"admin", "set-password", "--auth-file", "auth.json"},
	} {
		if _, err := ParseCommand(args); err == nil {
			t.Fatalf("ParseCommand(%v) accepted a path override", args)
		}
	}
}

func TestRunRejectsInvalidCommand(t *testing.T) {
	var stderr bytes.Buffer

	got := Run(context.Background(), []string{"serve", "--data-dir", "/tmp/data"}, nil, nil, &stderr)

	if got != 2 {
		t.Fatalf("Run() = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "用法：singbox-web serve | singbox-web admin set-password") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunServeReportsMissingPassword(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer

	got := Run(context.Background(), []string{"serve"}, nil, nil, &stderr)

	if got != 1 {
		t.Fatalf("Run() = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "singbox-web admin set-password") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
