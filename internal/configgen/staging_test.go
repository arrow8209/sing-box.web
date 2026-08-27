package configgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceStagingDirectoryRetainsLatestCompleteCandidate(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.staging-0123456789abcdef0123456789abcdef")
	generator := NewGenerator()
	first := Output{Files: []File{{Name: "10-log.json", Content: []byte(`{"log":{}}`)}, {Name: "80-route.json", Content: []byte(`{"route":{"final":"old"}}`)}}}
	if err := generator.ReplaceStagingDirectory(context.Background(), first, target); err != nil {
		t.Fatal(err)
	}
	second := Output{Files: []File{{Name: "80-route.json", Content: []byte(`{"route":{"final":"new"}}`)}}}
	if err := generator.ReplaceStagingDirectory(context.Background(), second, target); err != nil {
		t.Fatal(err)
	}
	if payload, err := os.ReadFile(filepath.Join(target, "80-route.json")); err != nil || string(payload) != `{"route":{"final":"new"}}` {
		t.Fatalf("route=%q err=%v", payload, err)
	}
	if _, err := os.Stat(filepath.Join(target, "10-log.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed module remains: %v", err)
	}
	for _, suffix := range []string{".preparing", ".previous"} {
		if _, err := os.Stat(target + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("replacement directory remains %s: %v", suffix, err)
		}
	}
}

func TestReplaceStagingDirectoryPreservesPreviousCandidateOnCanceledRefresh(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.staging-0123456789abcdef0123456789abcdef")
	if err := NewGenerator().ReplaceStagingDirectory(context.Background(), Output{Files: []File{{Name: "10-log.json", Content: []byte("old")}}}, target); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewGenerator().ReplaceStagingDirectory(ctx, Output{Files: []File{{Name: "10-log.json", Content: []byte("new")}}}, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if payload, err := os.ReadFile(filepath.Join(target, "10-log.json")); err != nil || string(payload) != "old" {
		t.Fatalf("previous candidate=%q err=%v", payload, err)
	}
}

func TestRecoverStagingDirectoriesRestoresCanonicalDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.staging-0123456789abcdef0123456789abcdef")
	previous := target + ".previous"
	preparing := target + ".preparing"
	for path, payload := range map[string]string{previous: "old", preparing: "partial"} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "10-log.json"), []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := RecoverStagingDirectories(root); err != nil {
		t.Fatal(err)
	}
	if payload, err := os.ReadFile(filepath.Join(target, "10-log.json")); err != nil || string(payload) != "old" {
		t.Fatalf("recovered=%q err=%v", payload, err)
	}
	for _, path := range []string{preparing, previous} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("replacement residue remains %s: %v", path, err)
		}
	}
}
