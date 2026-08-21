package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	password := []byte("correct horse battery staple")
	encoded, err := Hash(password, strings.NewReader("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("hash=%q", encoded)
	}
	if ok, err := Verify(password, encoded); err != nil || !ok {
		t.Fatalf("Verify() = %v, %v", ok, err)
	}
	if ok, err := Verify([]byte("incorrect password"), encoded); err != nil || ok {
		t.Fatalf("Verify(wrong) = %v, %v", ok, err)
	}
}

func TestHashRejectsInvalidPasswordLength(t *testing.T) {
	for _, password := range [][]byte{[]byte("short"), make([]byte, 1025)} {
		if _, err := Hash(password, strings.NewReader(strings.Repeat("x", 32))); err == nil {
			t.Fatalf("Hash(%d bytes) accepted invalid password", len(password))
		}
	}
	if _, err := Hash([]byte("六个字符测试"), strings.NewReader(strings.Repeat("x", 32))); err != nil {
		t.Fatalf("Hash() rejected six-rune password: %v", err)
	}
}

func TestVerifyRejectsExcessiveHashParameters(t *testing.T) {
	encoded, err := Hash([]byte("correct horse battery staple"), strings.NewReader("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	excessive := strings.Replace(encoded, "m=65536", "m=65537", 1)
	if _, err := Verify([]byte("correct horse battery staple"), excessive); err == nil {
		t.Fatal("Verify accepted excessive memory parameter")
	}
}
