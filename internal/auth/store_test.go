package auth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetPasswordStoresHashAndRotatesSessions(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
	randomBytes := append(bytes.Repeat([]byte{1}, 48), bytes.Repeat([]byte{2}, 48)...)
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), now, bytes.NewReader(randomBytes))
	if err := store.SetPassword([]byte("first-strong-password")); err != nil {
		t.Fatal(err)
	}
	first, ok, err := store.Authenticate([]byte("first-strong-password"))
	if err != nil || !ok {
		t.Fatalf("Authenticate() ok = %v, err = %v", ok, err)
	}
	if first.PasswordHash == "first-strong-password" || first.SessionKey == "" {
		t.Fatalf("unsafe record = %#v", first)
	}
	if err := store.SetPassword([]byte("second-strong-password")); err != nil {
		t.Fatal(err)
	}
	second, ok, err := store.Authenticate([]byte("second-strong-password"))
	if err != nil || !ok {
		t.Fatalf("second Authenticate() ok = %v, err = %v", ok, err)
	}
	if second.SessionEpoch != first.SessionEpoch+1 || second.SessionKey == first.SessionKey {
		t.Fatalf("session rotation: first=%#v second=%#v", first, second)
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), time.Now, bytes.NewReader(bytes.Repeat([]byte{1}, 48)))
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Authenticate([]byte("a-wrong-password")); err != nil || ok {
		t.Fatalf("Authenticate() ok = %v, err = %v", ok, err)
	}
}

func TestSetPasswordRejectsShortUnicodePassword(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), time.Now, bytes.NewReader(nil))
	if err := store.SetPassword([]byte(strings.Repeat("密", 11))); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("SetPassword() error = %v, want ErrWeakPassword", err)
	}
}

func TestSetPasswordReturnsPersistenceError(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewFileStore(filepath.Join(parent, "auth.json"), time.Now, bytes.NewReader(nil))
	if err := store.SetPassword([]byte("a-strong-password")); err == nil {
		t.Fatal("SetPassword() error = nil, want persistence error")
	}
}

func TestLoadParsesStoredPHCString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	store := NewFileStore(path, time.Now, bytes.NewReader(bytes.Repeat([]byte{1}, 48)))
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	reopened := NewFileStore(path, time.Now, bytes.NewReader(nil))
	record, err := reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(record.PasswordHash, "$argon2id$v=19$") {
		t.Fatalf("PasswordHash = %q, want PHC Argon2id string", record.PasswordHash)
	}
	if _, err := parsePHC(record.PasswordHash); err != nil {
		t.Fatalf("parsePHC() error = %v", err)
	}
}
