//go:build linux

package core

import (
	"io"
	"path/filepath"
	"testing"
)

func TestLinuxProcessFactoryReportsExit(t *testing.T) {
	process, err := (LinuxProcessFactory{}).Start(ProcessSpec{
		Path: "/bin/true", Dir: filepath.Dir("/bin/true"), Env: allowedEnvironment(), Stdout: io.Discard, Stderr: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := <-process.Exited()
	if !ok || result.Err != nil || result.ExitCode != 0 {
		t.Fatalf("result=%+v open=%t", result, ok)
	}
}
