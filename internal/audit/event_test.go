package audit_test

import (
	"bytes"
	"strings"
	"testing"

	"singbox-web/internal/audit"
)

func TestAuditLoggerAllowsMetadataButNotPayloads(t *testing.T) {
	var output bytes.Buffer
	logger := audit.NewLogger(&output)
	logger.Record(audit.Event{Operation: "save_draft", InternalID: "profile-a", Revision: 3, Digest: strings.Repeat("a", 64), Stage: "completed", DurationMS: 12, Result: "success"})
	text := output.String()
	for _, expected := range []string{"save_draft", "profile-a", `"revision":3`, `"result":"success"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
	for _, forbidden := range []string{"config", "password", "downloadBody", "url"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("forbidden field %q in %s", forbidden, text)
		}
	}
}
