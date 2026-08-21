package core_test

import (
	"bytes"
	"os"
	"path/filepath"
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
