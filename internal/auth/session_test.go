package auth

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{7}, 1024))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	session, err := manager.Login("admin", []byte("a-strong-password"))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Validate(session.Token)
	if err != nil || claims.Username != "admin" || claims.CSRFToken != session.CSRFToken {
		t.Fatalf("claims=%#v err=%v", claims, err)
	}
	if _, err := manager.Validate(session.Token + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	current = current.Add(24*time.Hour + time.Second)
	if _, err := manager.Validate(session.Token); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestPasswordChangeInvalidatesExistingSession(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{9}, 2048))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("first-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	session, err := manager.Login("admin", []byte("first-password"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPassword([]byte("second-password")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Validate(session.Token); err == nil {
		t.Fatal("pre-change session accepted")
	}
}

func TestSessionRefreshIsThrottledAndSlidesIdleExpiry(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{11}, 4096))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	original, err := manager.Login("admin", []byte("a-strong-password"))
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(14 * time.Minute)
	if _, renewed, err := manager.Refresh(original.Token); err != nil || renewed {
		t.Fatalf("14 minute refresh: renewed=%v err=%v", renewed, err)
	}
	current = current.Add(2 * time.Minute)
	refreshed, renewed, err := manager.Refresh(original.Token)
	if err != nil || !renewed {
		t.Fatalf("16 minute refresh: renewed=%v err=%v", renewed, err)
	}
	if refreshed.CSRFToken != original.CSRFToken || !refreshed.ExpiresAt.Equal(current.Add(24*time.Hour)) {
		t.Fatalf("refreshed=%#v original=%#v", refreshed, original)
	}
	current = original.ExpiresAt.Add(time.Second)
	if _, err := manager.Validate(original.Token); err == nil {
		t.Fatal("original idle token remained valid")
	}
	if _, err := manager.Validate(refreshed.Token); err != nil {
		t.Fatalf("active token expired: %v", err)
	}
}

func TestSessionBoundaryAndInvalidErrors(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{13}, 4096))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	if _, err := manager.Login("operator", []byte("a-strong-password")); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Login() error = %v, want ErrInvalidSession", err)
	}
	session, err := manager.Login(Username, []byte("a-strong-password"))
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(15 * time.Minute)
	if _, renewed, err := manager.Refresh(session.Token); err != nil || !renewed {
		t.Fatalf("15 minute refresh: renewed=%v err=%v", renewed, err)
	}
	current = session.ExpiresAt
	if _, err := manager.Validate(session.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Validate() at expiry error = %v, want ErrInvalidSession", err)
	}
	for _, token := range []string{"", ".", "a.b", "a.b.c", "%%%...."} {
		if _, err := manager.Validate(token); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("Validate(%q) error = %v, want ErrInvalidSession", token, err)
		}
	}
}

func FuzzValidateSession(f *testing.F) {
	f.Add("")
	f.Add("payload.signature")
	f.Fuzz(func(t *testing.T, token string) {
		store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), time.Now, bytes.NewReader(nil))
		manager := NewSessionManager(store, time.Now, bytes.NewReader(nil))
		_, _ = manager.Validate(token)
	})
}
