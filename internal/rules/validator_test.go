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
