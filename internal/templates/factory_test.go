package templates

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func newTestFactory(t *testing.T) *Factory {
	t.Helper()
	random := append(bytes.Repeat([]byte{0x11}, 48), bytes.Repeat([]byte{0x22}, 48)...)
	factory, err := NewFactory(bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	return factory
}

func TestFactoryCreatesUniqueSecretsAndCacheIDs(t *testing.T) {
	factory := newTestFactory(t)
	first, err := factory.NewProfileSeed(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := factory.NewProfileSeed(time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if first.ClashSecret == second.ClashSecret || len(first.ClashSecret) != 64 {
		t.Fatalf("secrets are not unique 32-byte hex values")
	}
	if first.CacheID == second.CacheID || len(first.CacheID) != 35 || first.CacheID[:3] != "ID_" {
		t.Fatalf("cache IDs are not unique: %q %q", first.CacheID, second.CacheID)
	}
}

func TestFactoryDoesNotReadTempAtRuntime(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	factory, err := NewFactory(bytes.NewReader(bytes.Repeat([]byte{0x22}, 48)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := factory.NewProfileSeed(time.Now()); err != nil {
		t.Fatal(err)
	}
}
