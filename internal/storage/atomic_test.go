package storage_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/storage"
)

func TestWriteJSONReplacesCompleteDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := storage.WriteJSON(path, map[string]int{"revision": 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := storage.ReadJSON[map[string]int](path)
	if err != nil || state["revision"] != 1 {
		t.Fatalf("state=%v err=%v", state, err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestReadJSONWrapsMalformedFileWithPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := storage.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := storage.ReadJSON[struct {
		Revision int `json:"revision"`
	}](path)
	var corrupt *storage.CorruptFileError
	if !errors.As(err, &corrupt) || !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %v, want CorruptFileError containing %q", err, path)
	}
}

func TestReadJSONRejectsUnknownFieldsAndTrailingValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	for _, payload := range [][]byte{[]byte(`{"unexpected":true}`), []byte(`{} {}`)} {
		if err := storage.WriteFile(path, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := storage.ReadJSON[struct{}](path)
		var corrupt *storage.CorruptFileError
		if !errors.As(err, &corrupt) {
			t.Fatalf("payload %q error = %v, want CorruptFileError", payload, err)
		}
	}
}
