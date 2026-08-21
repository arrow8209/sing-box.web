//go:build linux

package core

import (
	"errors"
	"os/exec"
	"runtime"
	"slices"
	"syscall"
)

// LinuxProcessFactory starts each core in its own process group. Locking the
// OS thread keeps Pdeathsig associated with the thread that waits for it.
type LinuxProcessFactory struct{}

type linuxProcess struct {
	cmd  *exec.Cmd
	pgid int
	exit chan ProcessResult
}

type startResult struct {
	process Process
	err     error
}

func (LinuxProcessFactory) Start(spec ProcessSpec) (Process, error) {
	ready := make(chan startResult, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		cmd := exec.Command(spec.Path, spec.Args...)
		cmd.Dir = spec.Dir
		cmd.Env = slices.Clone(spec.Env)
		cmd.Stdout, cmd.Stderr = spec.Stdout, spec.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
		if err := cmd.Start(); err != nil {
			ready <- startResult{err: err}
			return
		}
		process := &linuxProcess{cmd: cmd, pgid: cmd.Process.Pid, exit: make(chan ProcessResult, 1)}
		ready <- startResult{process: process}
		process.exit <- resultFromWait(cmd.Wait())
		close(process.exit)
	}()
	result := <-ready
	return result.process, result.err
}

func (p *linuxProcess) PID() int { return p.pgid }

func (p *linuxProcess) SignalGroup(signal syscall.Signal) error {
	if p == nil || p.pgid <= 0 {
		return errors.New("核心进程不可用")
	}
	return syscall.Kill(-p.pgid, signal)
}

func (p *linuxProcess) Exited() <-chan ProcessResult { return p.exit }

func resultFromWait(err error) ProcessResult {
	if err == nil {
		return ProcessResult{}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return ProcessResult{ExitCode: exit.ExitCode(), Err: err}
	}
	return ProcessResult{ExitCode: -1, Err: err}
}
