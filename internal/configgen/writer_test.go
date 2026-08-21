package configgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDirectoryCreatesOnlyNewDirectoryWithSafeModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	output := Output{Files: []File{{Name: "10-log.json", Content: []byte(`{"log":{}}\n`)}, {Name: "80-route.json", Content: []byte(`{"route":{}}\n`)}}}
	if err := NewGenerator().WriteDirectory(context.Background(), output, dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode=%o", info.Mode().Perm())
	}
	file, err := os.Stat(filepath.Join(dir, "10-log.json"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm() != 0o600 {
		t.Fatalf("file mode=%o", file.Mode().Perm())
	}
	if err := NewGenerator().WriteDirectory(context.Background(), output, dir); err == nil {
		t.Fatal("rewrote existing directory")
	}
}

func TestWriteDirectoryRejectsInjectedNamesAndLeavesExistingTargetUntouched(t *testing.T) {
	target := filepath.Join(t.TempDir(), "live-config")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, output := range []Output{
		{Files: []File{{Name: "../escape", Content: []byte("x")}}},
		{Files: []File{{Name: "unknown.json", Content: []byte("x")}}},
		{Files: []File{{Name: "10-log.json", Content: []byte("x")}, {Name: "10-log.json", Content: []byte("x")}}},
	} {
		if err := NewGenerator().WriteDirectory(context.Background(), output, target); err == nil {
			t.Fatal("accepted unsafe output")
		}
		got, err := os.ReadFile(marker)
		if err != nil || string(got) != "keep" {
			t.Fatalf("existing directory touched: %q, %v", got, err)
		}
	}
}

func TestWriteDirectoryCleansNewDirectoryWhenContextIsCanceled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	context, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewGenerator().WriteDirectory(context, Output{Files: []File{{Name: "10-log.json", Content: []byte("x")}}}, dir)
	if !errors.Is(err, context.Err()) {
		t.Fatalf("err=%v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("new directory remains: %v", err)
	}
}
