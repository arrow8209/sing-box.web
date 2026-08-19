// Package singbox contains the narrowly scoped interface used to inspect
// sing-box configurations. It deliberately does not expose an arbitrary
// command runner.
package singbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	// ErrUnavailable indicates that the configured sing-box executable cannot be started.
	ErrUnavailable = errors.New("sing-box unavailable")
	// ErrTimeout indicates that sing-box did not finish before the configured deadline.
	ErrTimeout = errors.New("sing-box command timed out")
	// ErrCheckFailed indicates that sing-box exited unsuccessfully.
	ErrCheckFailed = errors.New("sing-box check failed")
)

// CheckResult is the bounded output and outcome of a configuration check.
type CheckResult struct {
	Valid    bool          `json:"valid"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

// Runner executes only the safe sing-box version and configuration-check commands.
type Runner struct {
	path        string
	timeout     time.Duration
	outputLimit int
}

// NewRunner creates a runner for one configured sing-box binary.
func NewRunner(path string, timeout time.Duration, outputLimit int) *Runner {
	if outputLimit < 0 {
		outputLimit = 0
	}
	return &Runner{path: path, timeout: timeout, outputLimit: outputLimit}
}

// Version runs exactly "<path> version" and returns its first non-empty line.
func (r *Runner) Version(ctx context.Context) (string, error) {
	stdout, stderr, err := r.run(ctx, "", "version")
	if err != nil {
		return "", commandError(err, stdout, stderr)
	}
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line, nil
		}
	}
	return "", nil
}

// Check runs exactly "<path> check -C <dir>" with dir as the child working directory.
func (r *Runner) Check(ctx context.Context, dir string) (CheckResult, error) {
	started := time.Now()
	stdout, stderr, err := r.run(ctx, dir, "check", "-C", dir)
	result := CheckResult{
		Valid:    err == nil,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: time.Since(started),
	}
	if err != nil {
		return result, commandError(err, stdout, stderr)
	}
	return result, nil
}

func (r *Runner) run(ctx context.Context, dir string, args ...string) (string, string, error) {
	childCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	stdout := newCappedWriter(r.outputLimit)
	stderr := newCappedWriter(r.outputLimit)
	cmd := exec.CommandContext(childCtx, r.path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	out, errOut := Sanitize(stdout.String()), Sanitize(stderr.String())
	if err == nil {
		return out, errOut, nil
	}
	if errors.Is(childCtx.Err(), context.DeadlineExceeded) {
		return out, errOut, fmt.Errorf("%w: %v", ErrTimeout, childCtx.Err())
	}
	if isUnavailable(err) {
		return out, errOut, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return out, errOut, fmt.Errorf("%w: %v", ErrCheckFailed, err)
}

func isUnavailable(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr) || errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
}

func commandError(cause error, stdout, stderr string) error {
	parts := make([]string, 0, 2)
	if stdout != "" {
		parts = append(parts, "stdout="+stdout)
	}
	if stderr != "" {
		parts = append(parts, "stderr="+stderr)
	}
	if len(parts) == 0 {
		return cause
	}
	return fmt.Errorf("%w: %s", cause, strings.Join(parts, " "))
}

type cappedWriter struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func newCappedWriter(limit int) *cappedWriter {
	return &cappedWriter{limit: limit}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := w.limit - len(w.buf)
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		w.buf = append(w.buf, p[:remaining]...)
	}
	// Reporting the full write prevents os/exec from treating a deliberately
	// discarded suffix as a subprocess I/O failure.
	return len(p), nil
}

func (w *cappedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}

var _ io.Writer = (*cappedWriter)(nil)
