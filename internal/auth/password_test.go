package auth

import (
	"bytes"
	"strings"
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

func TestParsePHCRejectsDuplicateAndUnknownParameters(t *testing.T) {
	hash, err := HashPassword([]byte("a-strong-password"), bytes.NewReader(bytes.Repeat([]byte{1}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{
		strings.Replace(hash, "m=65536", "m=1,m=65536", 1),
		strings.Replace(hash, "p=2", "p=2,x=1", 1),
	} {
		if _, err := parsePHC(encoded); err == nil {
			t.Fatalf("parsePHC(%q) error = nil, want malformed-parameters error", encoded)
		}
	}
}
