package core_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/core"
	"singbox-web/internal/storage"
)

func TestCoreStoreRejectsCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "core.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := core.NewStore(path).Load()
	var corrupt *storage.CorruptFileError
	if !errors.As(err, &corrupt) || corrupt.Path != path {
		t.Fatalf("err=%v", err)
	}
}

func TestCoreStoreUpdateStartsAtVersionOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "core.json")
	state, err := core.NewStore(path).Update(func(state *core.PersistentState) error {
		state.ExpectedRunning = true
		return nil
	})
	if err != nil || state.Version != 1 || !state.ExpectedRunning {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}
