package rules

import (
	"bytes"
	"testing"
)

func TestValidateSourceRequiresRuleSetObject(t *testing.T) {
	for _, body := range [][]byte{nil, []byte(`[]`), []byte(`{"version":3}`), []byte(`{"version":3,"rules":[]}`)} {
		err := ValidateDownloaded(FormatSource, body)
		if len(body) > 0 && bytes.Contains(body, []byte(`"rules"`)) {
			if err != nil {
				t.Fatalf("valid body: %v", err)
			}
		} else if err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}

func TestValidateDownloadedBinaryRejectsSourceJSON(t *testing.T) {
	if err := ValidateDownloaded(FormatBinary, []byte(`{"version":3,"rules":[]}`)); err == nil {
		t.Fatal("accepted Source JSON as binary")
	}
	if err := ValidateDownloaded(FormatBinary, []byte("binary-rule-data")); err != nil {
		t.Fatalf("binary rejected: %v", err)
	}
}

func TestNormalizeManualSourceRequiresContentAndCountsValues(t *testing.T) {
	payload, count, err := NormalizeManualSource([]byte(`{"version":3,"rules":[{"domain_suffix":["example.com","example.org"]},{"ip_is_private":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || !bytes.HasSuffix(payload, []byte("\n")) || !bytes.Contains(payload, []byte("  \"rules\"")) {
		t.Fatalf("count=%d payload=%q", count, payload)
	}
	for _, body := range [][]byte{
		[]byte(`{"version":3,"rules":[]}`),
		[]byte(`{"version":0,"rules":[{}]}`),
		[]byte(`{"version":3,"rules":[{}]}`),
	} {
		if _, _, err := NormalizeManualSource(body); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
