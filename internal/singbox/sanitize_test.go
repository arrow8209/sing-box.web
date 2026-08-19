package singbox

import (
	"strings"
	"testing"
)

func TestSanitizeRedactsExactSensitiveKeysWithoutSwallowingNeighbors(t *testing.T) {
	in := `password=plain, status=ok
"ACCESS_TOKEN": "令牌值", next=still-here
authorization: Bearer abc.def, message=keep
uuid='abc-123'; other=value
not_secret=keep tokenization=keep`
	got := Sanitize(in)
	for _, secret := range []string{"plain", "令牌值", "abc.def", "abc-123"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
	for _, neighbor := range []string{"status=ok", "next=still-here", "message=keep", "other=value", "not_secret=keep", "tokenization=keep"} {
		if !strings.Contains(got, neighbor) {
			t.Fatalf("neighbor %q was swallowed in %q", neighbor, got)
		}
	}
	if strings.Count(got, "***REDACTED***") != 4 {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeHandlesNewlinesQuotesAndAllConfiguredKeys(t *testing.T) {
	keys := []string{"password", "passwd", "secret", "token", "access_token", "refresh_token", "authorization", "api_key", "client_secret", "uuid", "private_key", "pre_shared_key"}
	var input strings.Builder
	for _, key := range keys {
		input.WriteString("\n'")
		input.WriteString(strings.ToUpper(key))
		input.WriteString("': \"value-")
		input.WriteString(key)
		input.WriteString("\"")
	}
	got := Sanitize(input.String())
	for _, key := range keys {
		if strings.Contains(got, "value-"+key) {
			t.Fatalf("value for %s leaked: %q", key, got)
		}
	}
	if strings.Count(got, "***REDACTED***") != len(keys) {
		t.Fatalf("got %q", got)
	}
}
