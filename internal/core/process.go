package core

import (
	"io"
	"syscall"
)

// ProcessSpec deliberately contains only values derived by the server. It is
// never constructed from an HTTP request.
type ProcessSpec struct {
	Path   string
	Args   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type ProcessResult struct {
	ExitCode int
	Err      error
}

type Process interface {
	PID() int
	SignalGroup(syscall.Signal) error
	Exited() <-chan ProcessResult
}

type ProcessFactory interface {
	Start(ProcessSpec) (Process, error)
}
