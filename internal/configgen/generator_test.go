package configgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"singbox-web/internal/apierror"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

const resourceA = "0123456789abcdef0123456789abcdef"

func TestGenerateWritesOneDeterministicFilePerPresentModule(t *testing.T) {
	draft := profile.Draft{Config: map[string]json.RawMessage{
		"log":       json.RawMessage(`{"level":"info","future_log_field":{"enabled":true}}`),
		"outbounds": json.RawMessage(`[{"type":"future-protocol","tag":"node-a","future":{"x":1}}]`),
		"route":     json.RawMessage(`{"rules":[{"rule_set":"GeoSite-CN","outbound":"node-a"}]}`),
	}, Bindings: []profile.RuleBinding{{ResourceID: resourceA, Tag: "GeoSite-CN", Enabled: true}}}
	snapshot := rules.Snapshot{Resources: map[string]rules.Resource{resourceA: {ID: resourceA, Format: rules.FormatBinary, LocalPath: "data/rulesets/" + resourceA + ".srs"}}, Digests: map[string]string{resourceA: strings.Repeat("a", 64)}}
	first, err := NewGenerator().Generate(context.Background(), draft, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewGenerator().Generate(context.Background(), draft, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || !equalFiles(first.Files, second.Files) {
		t.Fatal("generation is not deterministic")
	}
	assertFileJSON(t, first.Files, "10-log.json", `{"log":{"future_log_field":{"enabled":true},"level":"info"}}`)
	assertFileContains(t, first.Files, "70-outbounds.json", `"future": {`)
	assertFileContains(t, first.Files, "80-route.json", `"type": "local"`)
	assertFileContains(t, first.Files, "80-route.json", `"path": "data/rulesets/`+resourceA+`.srs"`)
	assertNoFileContains(t, first.Files, "https://", "download_detour", "update_interval")
}

func TestGenerateOmitsMissingAndSchemaModules(t *testing.T) {
	output, err := NewGenerator().Generate(context.Background(), profile.Draft{Config: map[string]json.RawMessage{"$schema": json.RawMessage(`"https://example.test/schema"`), "dns": json.RawMessage(`null`)}}, rules.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Files) != 0 {
		t.Fatalf("files=%v", output.Files)
	}
}

func TestGenerateCanonicalizesObjectKeyOrderWithoutLosingNumbers(t *testing.T) {
	first := draftWithModule("log", `{"future":{"z":1,"a":9007199254740993},"level":"info"}`)
	second := draftWithModule("log", `{"level":"info","future":{"a":9007199254740993,"z":1}}`)
	outputA, err := NewGenerator().Generate(context.Background(), first, rules.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	outputB, err := NewGenerator().Generate(context.Background(), second, rules.Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outputA.Files[0].Content, outputB.Files[0].Content) {
		t.Fatalf("non-canonical output\n%s\n%s", outputA.Files[0].Content, outputB.Files[0].Content)
	}
	if !bytes.Contains(outputA.Files[0].Content, []byte("9007199254740993")) {
		t.Fatal("integer precision changed")
	}
}

func TestGenerateRejectsRemoteAndInvalidBindings(t *testing.T) {
	validSnapshot := rules.Snapshot{Resources: map[string]rules.Resource{resourceA: {ID: resourceA, Format: rules.FormatBinary, LocalPath: "data/rulesets/" + resourceA + ".srs"}}}
	for _, test := range []struct {
		name      string
		draft     profile.Draft
		snapshot  rules.Snapshot
		code      string
		fieldPath string
	}{
		{"remote", draftWithModule("route", `{"rule_set":[{"type":"remote","tag":"a","url":"https://example.test/a.srs"}]}`), validSnapshot, "remote_rule_set_forbidden", "route.rule_set[0].type"},
		{"duplicate manual", draftWithModule("route", `{"rule_set":[{"type":"local","tag":"a"},{"type":"inline","tag":"a"}]}`), validSnapshot, "duplicate_rule_set_tag", "route.rule_set[1].tag"},
		{"missing snapshot", profile.Draft{Bindings: []profile.RuleBinding{{ResourceID: resourceA, Tag: "a", Enabled: true}}}, rules.Snapshot{}, "rule_resource_not_found", "bindings[0].resourceId"},
		{"wrong path", profile.Draft{Bindings: []profile.RuleBinding{{ResourceID: resourceA, Tag: "a", Enabled: true}}}, rules.Snapshot{Resources: map[string]rules.Resource{resourceA: {ID: resourceA, Format: rules.FormatBinary, LocalPath: "/tmp/escaped.srs"}}}, "invalid_rule_resource_path", "bindings[0].resourceId"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGenerator().Generate(context.Background(), test.draft, test.snapshot)
			var public *apierror.HTTPError
			if !errors.As(err, &public) || public.Public.Code != test.code || public.Public.FieldPath != test.fieldPath {
				t.Fatalf("err=%#v", err)
			}
		})
	}
}

func TestGenerateAcceptsAbsoluteFixedSnapshotPath(t *testing.T) {
	draft := profile.Draft{Bindings: []profile.RuleBinding{{ResourceID: resourceA, Tag: "a", Enabled: true}}}
	snapshot := rules.Snapshot{Resources: map[string]rules.Resource{resourceA: {ID: resourceA, Format: rules.FormatSource, LocalPath: "/work/data/rulesets/" + resourceA + ".json"}}}
	output, err := NewGenerator().Generate(context.Background(), draft, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	assertFileContains(t, output.Files, "80-route.json", `"path": "data/rulesets/`+resourceA+`.json"`)
}

func draftWithModule(key, value string) profile.Draft {
	return profile.Draft{Config: map[string]json.RawMessage{key: json.RawMessage(value)}}
}

func equalFiles(left, right []File) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || !bytes.Equal(left[index].Content, right[index].Content) {
			return false
		}
	}
	return true
}

func assertFileJSON(t *testing.T, files []File, name, expected string) {
	t.Helper()
	for _, file := range files {
		if file.Name != name {
			continue
		}
		var gotValue, expectedValue any
		if err := json.Unmarshal(file.Content, &gotValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
			t.Fatal(err)
		}
		got, _ := json.Marshal(gotValue)
		want, _ := json.Marshal(expectedValue)
		if !bytes.Equal(got, want) {
			t.Fatalf("%s = %s, want %s", name, got, want)
		}
		return
	}
	t.Fatalf("file %s not found", name)
}

func assertFileContains(t *testing.T, files []File, name, want string) {
	t.Helper()
	for _, file := range files {
		if file.Name == name {
			if !bytes.Contains(file.Content, []byte(want)) {
				t.Fatalf("%s does not contain %q:\n%s", name, want, file.Content)
			}
			return
		}
	}
	t.Fatalf("file %s not found", name)
}

func assertNoFileContains(t *testing.T, files []File, forbidden ...string) {
	t.Helper()
	for _, file := range files {
		for _, value := range forbidden {
			if bytes.Contains(file.Content, []byte(value)) {
				t.Fatalf("%s contains forbidden %q", file.Name, value)
			}
		}
	}
}
