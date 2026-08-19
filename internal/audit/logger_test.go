package audit

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestLoggerEmitsOnlyConstrainedEventFields(t *testing.T) {
	var output bytes.Buffer
	now := time.Date(2026, 8, 19, 1, 2, 3, 0, time.UTC)
	logger := New(&output, func() time.Time { return now })
	logger.Record(Event{Operation: "publish", ProfileID: "abc", Revision: 4, Digest: "digest", DurationMS: 12, Result: "ok", RequestID: "request"})
	var got map[string]any
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 || got["timestamp"] != now.Format(time.RFC3339Nano) || got["operation"] != "publish" || got["result"] != "ok" {
		t.Fatalf("event=%v", got)
	}
}
