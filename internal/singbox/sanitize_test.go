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
			t.Fatalf("a sensitive value leaked")
		}
	}
	for _, neighbor := range []string{"status=ok", "next=still-here", "message=keep", "other=value", "not_secret=keep", "tokenization=keep"} {
		if !strings.Contains(got, neighbor) {
			t.Fatalf("an adjacent field was swallowed")
		}
	}
	if strings.Count(got, "***REDACTED***") != 4 {
		t.Fatal("unexpected redaction count")
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
			t.Fatalf("value for %s leaked", key)
		}
	}
	if strings.Count(got, "***REDACTED***") != len(keys) {
		t.Fatal("unexpected redaction count")
	}
}

func TestSanitizeRecognizesASCIIFieldBoundaries(t *testing.T) {
	in := `[password=square] (token=round) ?access_token=query&next=ok|uuid=pipe; private_key=semi, client_secret=comma status=keep`
	got := Sanitize(in)
	for _, secret := range []string{"square", "round", "query", "pipe", "semi", "comma"} {
		if strings.Contains(got, secret) {
			t.Fatal("a sensitive value leaked")
		}
	}
	for _, field := range []string{"&next=ok", "|", ";", ",", " status=keep"} {
		if !strings.Contains(got, field) {
			t.Fatal("a field separator or neighbor was swallowed")
		}
	}
	if !strings.Contains(got, "status=keep") {
		t.Fatal("ordinary adjacent field was redacted")
	}
}

func TestSanitizeDoesNotMatchKeySubstrings(t *testing.T) {
	in := `not_password=keep mytoken=value tokenized=unchanged (not_uuid=still)`
	if got := Sanitize(in); got != in {
		t.Fatal("ordinary key substring was redacted")
	}
}
