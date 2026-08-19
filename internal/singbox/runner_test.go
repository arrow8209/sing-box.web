package singbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeSingBox(t *testing.T, mode string) (string, string) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "fake-sing-box-"+mode)
	argsFile := binary + ".args"
	script := `#!/bin/sh
printf '%s\n' "$@" > "$0.args"
case "$0" in
  *success)
    if [ "$1" = version ]; then
      printf '\n  sing-box version 1.test  \n'
    else
      printf 'abcdefghijklmnopqrstuvwxyz'
    fi
    exit 0 ;;
  *failure)
    printf 'secret=do-not-leak\n' >&2
    exit 1 ;;
  *timeout)
    exec sleep 2 ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binary, argsFile
}

func TestRunnerCheckUsesOnlyCheckAndConfigDirectory(t *testing.T) {
	binary, argsFile := writeFakeSingBox(t, "success")
	runner := NewRunner(binary, time.Second, 64*1024)
	dir := t.TempDir()
	result, err := runner.Check(context.Background(), dir)
	if err != nil || !result.Valid {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(args), "check\n-C\n"+dir+"\n"; got != want {
		t.Fatalf("args=%q want=%q", got, want)
	}
}

func TestRunnerCheckCapsEachOutputAtConfiguredByteLimit(t *testing.T) {
	binary, _ := writeFakeSingBox(t, "success")
	result, err := NewRunner(binary, time.Second, 7).Check(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Stdout, "abcdefg"; got != want {
		t.Fatalf("stdout=%q want=%q", got, want)
	}
}

func TestRunnerCheckTimeout(t *testing.T) {
	binary, _ := writeFakeSingBox(t, "timeout")
	result, err := NewRunner(binary, 20*time.Millisecond, 64*1024).Check(context.Background(), t.TempDir())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err=%v, want ErrTimeout", err)
	}
	if result.Valid {
		t.Fatalf("result=%#v", result)
	}
}

func TestRunnerCheckFailureSanitizesErrorOutput(t *testing.T) {
	binary, _ := writeFakeSingBox(t, "failure")
	result, err := NewRunner(binary, time.Second, 64*1024).Check(context.Background(), t.TempDir())
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err=%v, want ErrCheckFailed", err)
	}
	if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(result.Stderr, "do-not-leak") {
		t.Fatalf("sensitive output leaked: result=%#v err=%v", result, err)
	}
	if !strings.Contains(err.Error(), "secret=***REDACTED***") {
		t.Fatalf("error does not contain sanitized stderr: %v", err)
	}
}

func TestRunnerVersionUsesOnlyVersionAndFirstNonemptyLine(t *testing.T) {
	binary, argsFile := writeFakeSingBox(t, "success")
	got, err := NewRunner(binary, time.Second, 64*1024).Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := "sing-box version 1.test"; got != want {
		t.Fatalf("version=%q want=%q", got, want)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(args), "version\n"; got != want {
		t.Fatalf("args=%q want=%q", got, want)
	}
}

func TestRunnerUnavailable(t *testing.T) {
	_, err := NewRunner(filepath.Join(t.TempDir(), "missing"), time.Second, 64).Version(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err=%v, want ErrUnavailable", err)
	}
}

func TestCheckResultRecordsDuration(t *testing.T) {
	binary, _ := writeFakeSingBox(t, "success")
	result, err := NewRunner(binary, time.Second, 64).Check(context.Background(), t.TempDir())
	if err != nil || result.Duration <= 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
