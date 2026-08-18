package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run(context.Background(), []string{"version"}, strings.NewReader(""), &stdout, io.Discard, Dependencies{Version: "0.1.0-test"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got, want := stdout.String(), "singbox-web 0.1.0-test\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
