package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONReplacesWholeFileAndSetsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, 0o600, map[string]any{"new": true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := ReadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got["new"] != true || got["old"] != nil {
		t.Fatalf("decoded state = %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestReadJSONRejectsTrailingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"ok":true}{"extra":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := ReadJSON(path, &got); err == nil {
		t.Fatal("ReadJSON() error = nil, want trailing-content error")
	}
}
