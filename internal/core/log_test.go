package core_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/core"
)

func TestBoundedLogLimitsLineAndTotalSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sing-box.log")
	log, err := core.NewBoundedLog(path, 4096, 128)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Write(append(bytes.Repeat([]byte("x"), 300), '\n')); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		if _, err := log.Write([]byte("normal line\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 4096 || bytes.Contains(payload, bytes.Repeat([]byte("x"), 129)) {
		t.Fatalf("log limits failed: %d", len(payload))
	}
}

func TestBoundedLogMirrorsNewLinesAndFlushesPartialLineOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sing-box.log")
	if err := os.WriteFile(path, []byte("old log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	log, err := core.NewBoundedLog(path, 4096, 128)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	var output bytes.Buffer
	log.SetOutput(&output)
	if _, err := log.Write([]byte("split ")); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("incomplete line was printed: %q", output.String())
	}
	if _, err := log.Write([]byte("line\n" + strings.Repeat("x", 300) + "\nlast line")); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "split line\n") || !strings.HasSuffix(output.String(), " [truncated]\n") {
		t.Fatalf("live output=%q", output.String())
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(persisted, append([]byte("old log\n"), output.Bytes()...)) || !strings.HasSuffix(output.String(), "last line\n") {
		t.Fatalf("persisted=%q output=%q", persisted, output.String())
	}
	for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
		if len(line)+1 > 128 {
			t.Fatalf("console line exceeds bound: %d", len(line))
		}
	}
}

type failedConsole struct{}

func (failedConsole) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestBoundedLogConsoleFailureDoesNotBreakCoreLogging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sing-box.log")
	log, err := core.NewBoundedLog(path, 4096, 128)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	log.SetOutput(failedConsole{})
	message := []byte("core output survives\n")
	if n, err := log.Write(message); n != len(message) || err != nil {
		t.Fatalf("Write()=(%d, %v)", n, err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(persisted, message) {
		t.Fatalf("persisted=%q err=%v", persisted, err)
	}
}
