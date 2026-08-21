package configgen

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type recordingRunner struct {
	spec    CommandSpec
	timeout time.Duration
	limit   int
	result  ProcessResult
	err     error
}

func (r *recordingRunner) Run(_ context.Context, spec CommandSpec, timeout time.Duration, limit int) (ProcessResult, error) {
	r.spec, r.timeout, r.limit = spec, timeout, limit
	return r.result, r.err
}

func TestCheckerUsesArgumentArrayAndBoundsEachStream(t *testing.T) {
	work := t.TempDir()
	binary := filepath.Join(work, "sing-box")
	if err := os.WriteFile(binary, []byte("test"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{result: ProcessResult{Stdout: bytes.Repeat([]byte("o"), 70<<10), Stderr: bytes.Repeat([]byte("e"), 70<<10)}}
	checker := NewChecker(work, runner, 30*time.Second, 64<<10)
	result, err := checker.Check(context.Background(), binary, filepath.Join(work, "config.staging-a1"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{binary, "check", "-C", filepath.Join(work, "config.staging-a1")}
	if !reflect.DeepEqual(runner.spec.Args, want) || runner.spec.Dir != work {
		t.Fatalf("spec=%+v want args=%v dir=%s", runner.spec, want, work)
	}
	if runner.timeout != 30*time.Second || runner.limit != 64<<10 {
		t.Fatalf("timeout=%v limit=%d", runner.timeout, runner.limit)
	}
	if len(result.Stdout) != 64<<10 || len(result.Stderr) != 64<<10 || !result.OutputTruncated {
		t.Fatalf("result=%+v", result)
	}
}

func TestExecRunnerBoundsEachStream(t *testing.T) {
	result, err := (ExecRunner{}).Run(context.Background(), CommandSpec{Args: []string{"/bin/sh", "-c", "yes o | head -c 70000; yes e | head -c 70000 >&2"}, Dir: t.TempDir()}, 5*time.Second, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Stdout) != 64<<10 || len(result.Stderr) != 64<<10 || !result.OutputTruncated {
		t.Fatalf("result=%+v", result)
	}
}

func TestExecRunnerReportsTimeout(t *testing.T) {
	result, err := (ExecRunner{}).Run(context.Background(), CommandSpec{Args: []string{"/bin/sh", "-c", "sleep 2"}, Dir: t.TempDir()}, 10*time.Millisecond, 1024)
	if !errors.Is(err, ErrCheckTimeout) || !result.TimedOut {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestCheckerRejectsMissingOrNonExecutableCore(t *testing.T) {
	checker := NewChecker(t.TempDir(), &recordingRunner{}, time.Second, 1024)
	if _, err := checker.Check(context.Background(), filepath.Join(t.TempDir(), "missing"), "config"); !errors.Is(err, ErrCoreNotInstalled) {
		t.Fatalf("err=%v", err)
	}
	file := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Check(context.Background(), file, "config"); !errors.Is(err, ErrCoreNotInstalled) {
		t.Fatalf("err=%v", err)
	}
}
