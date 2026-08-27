package configgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var (
	ErrCoreNotInstalled = errors.New("core_not_installed")
	ErrCheckTimeout     = errors.New("singbox_check_timeout")
	ErrCheckFailed      = errors.New("singbox_check_failed")
)

type CommandSpec struct {
	Args []string
	Dir  string
}

type ProcessResult struct {
	Stdout          []byte
	Stderr          []byte
	ExitCode        int
	TimedOut        bool
	OutputTruncated bool
}

type CheckResult struct {
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExitCode        int    `json:"exitCode"`
	TimedOut        bool   `json:"timedOut"`
	OutputTruncated bool   `json:"outputTruncated"`
}

type Runner interface {
	Run(ctx context.Context, spec CommandSpec, timeout time.Duration, limit int) (ProcessResult, error)
}

// ExecRunner executes an argument array directly. It intentionally never
// invokes a shell and starts with a small fixed environment. Environment is
// an explicit allow-list extension, chiefly for controlled test helpers; it
// never inherits the web process environment.
type ExecRunner struct {
	Environment []string
}

func (r ExecRunner) Run(ctx context.Context, spec CommandSpec, timeout time.Duration, limit int) (ProcessResult, error) {
	if len(spec.Args) == 0 || spec.Args[0] == "" {
		return ProcessResult{}, fmt.Errorf("检查命令为空")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = append([]string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}, r.Environment...)
	// A check may invoke helper processes. Put it in a private process group so
	// a timeout cannot leave a child holding stdout/stderr open and delay Wait.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	stdout, stderr := newLimitedBuffer(limit), newLimitedBuffer(limit)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	result := ProcessResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode(err), OutputTruncated: stdout.Truncated() || stderr.Truncated()}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		return result, ErrCheckTimeout
	}
	return result, err
}

type limitedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	if limit < 0 {
		limit = 0
	}
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		b.data = append(b.data, value[:remaining]...)
	}
	if remaining < len(value) {
		b.truncated = true
	}
	return len(value), nil
}
func (b *limitedBuffer) Bytes() []byte   { return append([]byte(nil), b.data...) }
func (b *limitedBuffer) Truncated() bool { return b.truncated }

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		return exited.ExitCode()
	}
	return -1
}

type Checker struct {
	workDir string
	runner  Runner
	timeout time.Duration
	limit   int
}

func NewChecker(workDir string, runner Runner, timeout time.Duration, limit int) *Checker {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Checker{workDir: workDir, runner: runner, timeout: timeout, limit: limit}
}

// CheckCommand returns the exact argument array used for a core check. Every
// path is relative to workDir: the process already runs there, and keeping
// absolute host paths out of argv also keeps them out of core diagnostics.
func CheckCommand(workDir, binaryPath, configDir string) ([]string, error) {
	_, binary, err := relativeCheckPath(workDir, binaryPath)
	if err != nil {
		return nil, err
	}
	_, config, err := relativeCheckPath(workDir, configDir)
	if err != nil {
		return nil, err
	}
	return []string{binary, "check", "-C", config}, nil
}

func (c *Checker) Check(ctx context.Context, binaryPath, configDir string) (CheckResult, error) {
	absoluteBinary, _, pathErr := relativeCheckPath(c.workDir, binaryPath)
	if pathErr != nil {
		return CheckResult{}, fmt.Errorf("%w: 检查路径必须位于服务工作目录", ErrCheckFailed)
	}
	command, pathErr := CheckCommand(c.workDir, binaryPath, configDir)
	if pathErr != nil {
		return CheckResult{}, fmt.Errorf("%w: 检查路径必须位于服务工作目录", ErrCheckFailed)
	}
	info, err := os.Lstat(absoluteBinary)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return CheckResult{}, ErrCoreNotInstalled
	}
	result, err := c.runner.Run(ctx, CommandSpec{Args: command, Dir: c.workDir}, c.timeout, c.limit)
	outputTruncated := result.OutputTruncated || len(result.Stdout) > c.limit || len(result.Stderr) > c.limit
	result.Stdout, result.Stderr, result.OutputTruncated = boundOutput(result.Stdout, c.limit), boundOutput(result.Stderr, c.limit), outputTruncated
	checked := CheckResult{Stdout: redactWorkDirectory(string(result.Stdout), c.workDir), Stderr: redactWorkDirectory(string(result.Stderr), c.workDir), ExitCode: result.ExitCode, TimedOut: result.TimedOut, OutputTruncated: result.OutputTruncated}
	if errors.Is(err, ErrCheckTimeout) {
		return checked, ErrCheckTimeout
	}
	if err != nil {
		return checked, fmt.Errorf("%w: %v", ErrCheckFailed, err)
	}
	return checked, nil
}

func relativeCheckPath(workDir, path string) (string, string, error) {
	workDirectory, err := filepath.Abs(workDir)
	if err != nil {
		return "", "", err
	}
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(workDirectory, absolute)
	}
	absolute, err = filepath.Abs(absolute)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(workDirectory, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("检查路径越出服务工作目录")
	}
	return absolute, filepath.ToSlash(relative), nil
}

func redactWorkDirectory(value, workDir string) string {
	if value == "" || workDir == "" {
		return value
	}
	clean := strings.TrimRight(filepath.Clean(workDir), string(filepath.Separator))
	if clean == "" || clean == "." || !filepath.IsAbs(clean) {
		return value
	}
	value = strings.ReplaceAll(value, clean+string(filepath.Separator), "")
	value = strings.ReplaceAll(value, filepath.ToSlash(clean)+"/", "")
	return strings.ReplaceAll(value, clean, ".")
}

func boundOutput(value []byte, limit int) []byte {
	if limit < 0 {
		limit = 0
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
