package auth

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestSessionRejectsExpiredAndPreviousGeneration(t *testing.T) {
	manager := NewSessionManager(nil)
	state := State{SessionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), SessionGeneration: 2}
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	token, _, err := manager.Issue(state, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := manager.Validate(token, state, now.Add(24*time.Hour)); ok {
		t.Fatal("expired session was accepted")
	}
	state.SessionGeneration++
	if _, _, ok := manager.Validate(token, state, now); ok {
		t.Fatal("previous generation was accepted")
	}
}

func TestSessionRefreshKeepsCSRFStable(t *testing.T) {
	manager := NewSessionManager(nil)
	state := State{SessionKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), SessionGeneration: 1}
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	token, csrf, err := manager.Issue(state, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, _, ok := manager.Validate(token, state, now.Add(6*time.Minute))
	if !ok {
		t.Fatal("issued session was rejected")
	}
	_, refreshedCSRF, err := manager.Refresh(state, claims, now.Add(6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if csrf != refreshedCSRF {
		t.Fatal("refresh changed csrf token")
	}
}
