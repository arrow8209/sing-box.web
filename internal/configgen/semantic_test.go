package configgen

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

func TestValidateSemanticReportsStablePaths(t *testing.T) {
	tests := []struct {
		name       string
		draft      profile.Draft
		snapshot   rules.Snapshot
		code, path string
	}{
		{"invalid buffer", profile.Draft{Buffers: map[string]string{"outbounds": "[{"}}, rules.Snapshot{}, "invalid_json_buffer", "outbounds"},
		{"unsupported endpoint module", semanticDraft("endpoints", `[]`), rules.Snapshot{}, "unknown_module", "endpoints"},
		{"wrong container", semanticDraft("outbounds", `{}`), rules.Snapshot{}, "invalid_module_container", "outbounds"},
		{"duplicate outbound", semanticDraft("outbounds", `[{"type":"direct","tag":"x"},{"type":"block","tag":"x"}]`), rules.Snapshot{}, "duplicate_tag", "outbounds[1].tag"},
		{"missing route outbound", semanticModules(map[string]string{"outbounds": `[{"type":"direct","tag":"direct"}]`, "route": `{"final":"missing"}`}), rules.Snapshot{}, "unknown_outbound", "route.final"},
		{"missing dns server", semanticDraft("dns", `{"servers":[{"type":"udp","tag":"dns-a","server":"1.1.1.1"}],"final":"dns-missing"}`), rules.Snapshot{}, "unknown_dns_server", "dns.final"},
		{"remote rule set", semanticDraft("route", `{"rule_set":[{"type":"remote","tag":"remote-a","url":"https://example.test/a.srs"}]}`), rules.Snapshot{}, "remote_rule_set_forbidden", "route.rule_set[0].type"},
		{"missing binding", profile.Draft{Bindings: []profile.RuleBinding{{ResourceID: resourceA, Tag: "GeoSite-CN", Enabled: true}}}, rules.Snapshot{}, "unknown_rule_resource", "bindings[0].resourceId"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSemanticError(t, ValidateSemantic(test.draft, test.snapshot), test.code, test.path)
		})
	}
}

func TestValidateSemanticBindingSnapshotAndReferences(t *testing.T) {
	now := time.Now()
	snapshot := rules.Snapshot{Resources: map[string]rules.Resource{resourceA: {ID: resourceA, LastSuccessAt: &now}}, Digests: map[string]string{resourceA: strings.Repeat("a", 64)}}
	draft := semanticModules(map[string]string{
		"outbounds": `[{"type":"direct","tag":"ok"}]`,
		"route":     `{"rule_set":[{"type":"local","tag":"manual"}],"rules":[{"rule_set":["manual","missing"],"outbound":"missing"}]}`,
	})
	draft.Bindings = []profile.RuleBinding{{ResourceID: resourceA, Tag: "bound", Enabled: true}, {ResourceID: resourceA, Tag: "bound", Enabled: true}}
	errors := ValidateSemantic(draft, snapshot)
	assertSemanticError(t, errors, "unknown_rule_set", "route.rules[0].rule_set[1]")
	assertSemanticError(t, errors, "unknown_outbound", "route.rules[0].outbound")
	assertSemanticError(t, errors, "duplicate_tag", "bindings[1].tag")
	for index := 1; index < len(errors); index++ {
		previous, current := errors[index-1], errors[index]
		if previous.FieldPath > current.FieldPath || previous.FieldPath == current.FieldPath && previous.Code > current.Code {
			t.Fatalf("errors not stable: %#v", errors)
		}
	}
}

func TestValidateSemanticUnknownFieldsAreIgnoredAndDraftIsUntouched(t *testing.T) {
	raw := `[{"type":"future","tag":"future","large":9007199254740993,"nested":{"unknown":true}}]`
	draft := profile.Draft{Config: map[string]json.RawMessage{"outbounds": json.RawMessage(raw)}, Buffers: map[string]string{"outbounds": raw}}
	before := string(draft.Config["outbounds"])
	if errors := ValidateSemantic(draft, rules.Snapshot{}); len(errors) != 0 {
		t.Fatalf("errors=%#v", errors)
	}
	if string(draft.Config["outbounds"]) != before || draft.Buffers["outbounds"] != raw {
		t.Fatal("semantic validation modified draft")
	}
}

func semanticDraft(key, raw string) profile.Draft {
	return semanticModules(map[string]string{key: raw})
}
func semanticModules(values map[string]string) profile.Draft {
	config := make(map[string]json.RawMessage, len(values))
	buffers := make(map[string]string, len(values))
	for key, value := range values {
		config[key] = json.RawMessage(value)
		buffers[key] = value
	}
	return profile.Draft{Config: config, Buffers: buffers}
}
func assertSemanticError(t *testing.T, errors []apierror.Error, code, path string) {
	t.Helper()
	for _, err := range errors {
		if err.Code == code && err.FieldPath == path && err.Stage == semanticStage {
			return
		}
	}
	t.Fatalf("missing %s at %s: %#v", code, path, errors)
}
