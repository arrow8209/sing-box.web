package validation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/profile"
)

func TestEmbeddedSchemaAcceptsDefaultConfig(t *testing.T) {
	validator, err := NewSchemaValidator(assets.SingBoxSchema)
	if err != nil {
		t.Fatal(err)
	}
	if issues := validator.Validate(assets.DefaultConfig); len(issues) != 0 {
		t.Fatalf("default config schema issues = %#v", issues)
	}
	if issues := ValidateSemantics(assets.DefaultConfig); len(issues) != 0 {
		t.Fatalf("default config semantic issues = %#v", issues)
	}
}

func TestEmbeddedSchemaMetadataMatchesPinnedAsset(t *testing.T) {
	var metadata struct {
		TargetVersion string `json:"targetVersion"`
		SchemaURL     string `json:"schemaURL"`
		SchemaCommit  string `json:"schemaCommit"`
		SHA256        string `json:"sha256"`
	}
	if err := json.Unmarshal(assets.SchemaVersion, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.TargetVersion != "1.13.19" {
		t.Fatalf("targetVersion = %q", metadata.TargetVersion)
	}
	if metadata.SchemaCommit != "583a690e888cdf6e8b024bf017ebf6297e08338d" {
		t.Fatalf("schemaCommit = %q", metadata.SchemaCommit)
	}
	if metadata.SchemaURL != "https://raw.githubusercontent.com/SagerNet/sing-box/"+metadata.SchemaCommit+"/schema.json" {
		t.Fatalf("schemaURL = %q", metadata.SchemaURL)
	}
	digest := sha256.Sum256(assets.SingBoxSchema)
	if got := fmt.Sprintf("%x", digest); metadata.SHA256 != got {
		t.Fatalf("schema sha256 = %q, metadata = %q", got, metadata.SHA256)
	}
}

func TestSchemaValidatorReportsJSONPointer(t *testing.T) {
	schema := []byte(`{
		"type":"object",
		"properties":{"log":{"type":"object","properties":{"level":{"enum":["debug","info","warn","error"]}}}},
		"required":["log"]
	}`)
	validator, err := NewSchemaValidator(schema)
	if err != nil {
		t.Fatal(err)
	}

	issues := validator.Validate(json.RawMessage(`{"log":{"level":"trace"}}`))
	if len(issues) != 1 {
		t.Fatalf("issues = %#v", issues)
	}
	if issues[0].Stage != "schema" || issues[0].Path != "/log/level" {
		t.Fatalf("issue = %#v", issues[0])
	}
	if strings.Contains(issues[0].Message, "trace") {
		t.Fatalf("message leaked rejected value: %q", issues[0].Message)
	}
	if issues := validator.Validate(json.RawMessage(`{"log":{"level":"info"}}`)); len(issues) != 0 {
		t.Fatalf("valid issues = %#v", issues)
	}
}

func TestServiceValidateDraftBlocksNonpublishableBuffers(t *testing.T) {
	validator, err := NewSchemaValidator([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 19, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	service := NewService(validator, "1.13.19", func() time.Time { return now })
	draft := profile.Draft{Content: profile.DraftContent{
		Config: json.RawMessage(`{}`),
		EditorBuffers: map[string]profile.EditorBuffer{
			"route/~draft": {Text: "private invalid content", Error: "private decoder error", Valid: false},
			"dns/server":   {Text: `{}`, Error: "private mismatch detail", Valid: true, Applied: false},
		},
	}}

	result := service.ValidateDraft(draft)
	if result.Valid || result.TargetVersion != "1.13.19" || result.InstalledVersion != "" || !result.CheckedAt.Equal(now.UTC()) {
		t.Fatalf("result metadata = %#v", result)
	}
	want := []Issue{
		{Stage: "json", Path: "/dns~1server", Code: "buffer_out_of_sync", Message: "原始 JSON 尚未应用到配置"},
		{Stage: "json", Path: "/route~1~0draft", Code: "invalid_json", Message: "原始 JSON 尚未成功解析"},
	}
	if len(result.Issues) != len(want) {
		t.Fatalf("issues = %#v", result.Issues)
	}
	for index := range want {
		if result.Issues[index] != want[index] {
			t.Fatalf("issues[%d] = %#v, want %#v", index, result.Issues[index], want[index])
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private invalid content", "private decoder error", "private mismatch detail"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("result leaked buffer data %q: %s", secret, encoded)
		}
	}
}

func TestServiceValidateDraftDelegatesWhenBuffersAreApplied(t *testing.T) {
	validator, err := NewSchemaValidator([]byte(`{"type":"object","required":["log"]}`))
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(validator, "1.13.19", func() time.Time { return time.Time{} })
	draft := profile.Draft{Content: profile.DraftContent{
		Config: json.RawMessage(`{}`),
		EditorBuffers: map[string]profile.EditorBuffer{
			"log": {Text: `{}`, Valid: true, Applied: true},
		},
	}}

	result := service.ValidateDraft(draft)
	if result.Valid || len(result.Issues) == 0 || result.Issues[0].Stage != "schema" {
		t.Fatalf("result = %#v", result)
	}
}
