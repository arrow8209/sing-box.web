package auth

import (
	"bytes"
	"testing"
)

func TestHashPasswordAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword([]byte("a-strong-password"), bytes.NewReader(bytes.Repeat([]byte{1}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword([]byte("a-strong-password"), hash); err != nil || !ok {
		t.Fatalf("VerifyPassword() ok = %v, err = %v", ok, err)
	}
}
