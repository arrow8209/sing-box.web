package app

import "testing"

func TestParseCommand(t *testing.T) {
  tests := []struct {
    args []string
    want Command
  }{
    {[]string{"serve"}, CommandServe},
    {[]string{"admin", "set-password"}, CommandSetPassword},
  }
  for _, test := range tests {
    got, err := ParseCommand(test.args)
    if err != nil || got != test.want {
      t.Fatalf("ParseCommand(%v) = %q, %v", test.args, got, err)
    }
  }
}

func TestParseCommandRejectsPathFlags(t *testing.T) {
  for _, args := range [][]string{
    {"serve", "--data-dir", "/tmp/data"},
    {"admin", "set-password", "--auth-file", "auth.json"},
  } {
    if _, err := ParseCommand(args); err == nil {
      t.Fatalf("ParseCommand(%v) accepted a path override", args)
    }
  }
}
