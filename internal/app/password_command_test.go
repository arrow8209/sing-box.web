package app

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
)

type testPasswordPrompter struct {
	values [][]byte
	err    error
}

func (p *testPasswordPrompter) ReadPassword(string) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	value := p.values[0]
	p.values = p.values[1:]
	return value, nil
}

func TestSetPasswordCommandNeverAcceptsPasswordArgument(t *testing.T) {
	if _, err := ParseCommand([]string{"admin", "set-password", "secret"}); err == nil {
		t.Fatal("password argument accepted")
	}
}

func TestRunSetPasswordStoresConfirmedPassword(t *testing.T) {
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	prompt := &testPasswordPrompter{values: [][]byte{[]byte("correct horse battery staple"), []byte("correct horse battery staple")}}
	var stdout bytes.Buffer
	if err := runSetPassword(layout, prompt, &stdout); err != nil {
		t.Fatal(err)
	}
	state, err := auth.NewStore(layout.AuthFile, nil).Load()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := auth.Verify([]byte("correct horse battery staple"), state.PasswordHash); err != nil || !ok {
		t.Fatalf("password was not stored: %v, %v", ok, err)
	}
	if filepath.Dir(layout.AuthFile) == "" || stdout.Len() == 0 {
		t.Fatal("missing output or auth file path")
	}
}

func TestRunSetPasswordRejectsMismatchedConfirmation(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = runSetPassword(layout, &testPasswordPrompter{values: [][]byte{[]byte("correct horse battery staple"), []byte("a second sufficiently long password")}}, &bytes.Buffer{})
	if !errors.Is(err, errPasswordsDoNotMatch) {
		t.Fatalf("error = %v", err)
	}
}
