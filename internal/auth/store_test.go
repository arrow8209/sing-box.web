package auth

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "auth.json"), rand.Reader)
}

func TestSetPasswordStoresArgon2idAndRotatesSessions(t *testing.T) {
	store := newTestStore(t)
	if err := store.SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	first, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.PasswordHash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("hash=%q", first.PasswordHash)
	}
	if ok, err := Verify([]byte("correct horse battery staple"), first.PasswordHash); err != nil || !ok {
		t.Fatal("password did not verify")
	}
	if err := store.SetPassword([]byte("a second sufficiently long password")); err != nil {
		t.Fatal(err)
	}
	second, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionGeneration != first.SessionGeneration+1 || second.SessionKey == first.SessionKey {
		t.Fatal("sessions were not invalidated")
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(store.file), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions=%#o", info.Mode().Perm())
	}
}
