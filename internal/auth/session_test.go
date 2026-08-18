package auth

import (
	"bytes"
	"encoding/base64"
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

func TestSessionSubsecondRefreshDoesNotRenewEarly(t *testing.T) {
	start := time.Date(2026, 8, 18, 12, 0, 0, int(900*time.Millisecond), time.UTC)
	current := start
	random := bytes.NewReader(bytes.Repeat([]byte{17}, 4096))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	session, err := manager.Login(Username, []byte("a-strong-password"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Validate(session.Token); err != nil {
		t.Fatalf("fresh subsecond session rejected: %v", err)
	}
	current = start.Add(14*time.Minute + 59*time.Second + 100*time.Millisecond)
	if _, renewed, err := manager.Refresh(session.Token); err != nil || renewed {
		t.Fatalf("14m59.1s refresh: renewed=%v err=%v", renewed, err)
	}
	current = start.Add(15*time.Minute + 100*time.Millisecond)
	if _, renewed, err := manager.Refresh(session.Token); err != nil || !renewed {
		t.Fatalf("15m0.1s refresh: renewed=%v err=%v", renewed, err)
	}
}

func TestValidateRejectsInvalidSignedClaimTimes(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{19}, 4096))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	record, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	now := current.Unix()
	tests := []struct {
		name      string
		issuedAt  int64
		expiresAt int64
	}{
		{name: "future", issuedAt: now + 2, expiresAt: now + 2 + int64(24*time.Hour/time.Second)},
		{name: "negative", issuedAt: -1, expiresAt: -1 + int64(24*time.Hour/time.Second)},
		{name: "not after issue", issuedAt: now, expiresAt: now},
		{name: "not 24 hours", issuedAt: now, expiresAt: now + int64(24*time.Hour/time.Second) + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := Claims{
				Username:  Username,
				Epoch:     record.SessionEpoch,
				IssuedAt:  test.issuedAt,
				ExpiresAt: test.expiresAt,
				CSRFToken: "test-csrf-token",
			}
			session, err := sessionFromClaims(claims, record.SessionKey)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Validate(session.Token); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("Validate() error = %v, want ErrInvalidSession", err)
			}
		})
	}
}

func TestValidateRejectsSessionAfterClockRollback(t *testing.T) {
	current := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{23}, 4096))
	store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	session, err := manager.Login(Username, []byte("a-strong-password"))
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(-48 * time.Hour)
	if _, err := manager.Validate(session.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Validate() after rollback error = %v, want ErrInvalidSession", err)
	}
}

func FuzzValidateSession(f *testing.F) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{29}, 4096))
	store := NewFileStore(filepath.Join(f.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		f.Fatal(err)
	}
	record, err := store.Load()
	if err != nil {
		f.Fatal(err)
	}
	manager := NewSessionManager(store, func() time.Time { return current }, random)
	f.Add("", int64(0), int64(0))
	f.Add("payload.signature", current.Unix(), current.Add(24*time.Hour).Unix())
	f.Fuzz(func(t *testing.T, token string, issuedAt int64, expiresAt int64) {
		_, _ = manager.Validate(token)

		payloadText := base64.RawURLEncoding.EncodeToString([]byte(token))
		key, err := decodeSessionKey(record.SessionKey)
		if err != nil {
			t.Fatal(err)
		}
		signedRaw := payloadText + "." + base64.RawURLEncoding.EncodeToString(signPayload(payloadText, key))
		clear(key)
		_, _ = manager.Validate(signedRaw)

		claims := Claims{
			Username:  Username,
			Epoch:     record.SessionEpoch,
			IssuedAt:  issuedAt,
			ExpiresAt: expiresAt,
			CSRFToken: "fuzz-csrf-token",
		}
		structured, err := sessionFromClaims(claims, record.SessionKey)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = manager.Validate(structured.Token)
	})
}
