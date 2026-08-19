package validation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateSemanticsReferences(t *testing.T) {
	tests := []struct {
		name string
		json string
		code string
		path string
	}{
		{"duplicate outbound tag", `{"outbounds":[{"tag":"x","type":"direct"},{"tag":"x","type":"block"}]}`, "duplicate_tag", "/outbounds/1/tag"},
		{"missing selector member", `{"outbounds":[{"tag":"select","type":"selector","outbounds":["missing"]}]}`, "missing_outbound", "/outbounds/0/outbounds/0"},
		{"missing route final", `{"outbounds":[{"tag":"direct","type":"direct"}],"route":{"final":"missing"}}`, "missing_outbound", "/route/final"},
		{"missing DNS server", `{"dns":{"servers":[{"tag":"local","type":"udp","server":"1.1.1.1"}],"final":"missing"}}`, "missing_dns_server", "/dns/final"},
		{"missing rule set", `{"route":{"rule_set":[],"rules":[{"rule_set":["missing"],"outbound":"direct"}]},"outbounds":[{"tag":"direct","type":"direct"}]}`, "missing_rule_set", "/route/rules/0/rule_set/0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := ValidateSemantics(json.RawMessage(test.json))
			if len(issues) != 1 {
				t.Fatalf("issues = %#v", issues)
			}
			if issues[0].Stage != "semantic" || issues[0].Code != test.code || issues[0].Path != test.path {
				t.Fatalf("issue = %#v", issues[0])
			}
		})
	}
}

func TestValidateSemanticsAllReferenceLocationsAndStableOrdering(t *testing.T) {
	config := json.RawMessage(`{
		"inbounds":[{"tag":"in"},{"tag":"in"}],
		"outbounds":[
			{"tag":"out","type":"direct","detour":"missing-detour"},
			{"tag":"test","type":"urltest","outbounds":["missing-member"]}
		],
		"dns":{
			"servers":[
				{"tag":"dns","domain_resolver":"missing-domain"},
				{"tag":"dns","domain_resolver":{"server":"missing-object"}}
			],
			"rules":[{"server":"missing-rule"}]
		},
		"route":{
			"rule_set":[{"tag":"set"},{"tag":"set"}],
			"rules":[{"outbound":"missing-route"}],
			"default_domain_resolver":{"server":"missing-default"}
		}
	}`)

	issues := ValidateSemantics(config)
	if len(issues) != 10 {
		t.Fatalf("issues = %#v", issues)
	}
	for index := 1; index < len(issues); index++ {
		previous, current := issues[index-1], issues[index]
		if previous.Path > current.Path || previous.Path == current.Path && previous.Code > current.Code {
			t.Fatalf("issues are not sorted: %#v", issues)
		}
	}
	encoded, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	for _, rejected := range []string{"missing-detour", "missing-member", "missing-domain", "missing-object", "missing-rule", "missing-route", "missing-default"} {
		if strings.Contains(string(encoded), rejected) {
			t.Fatalf("issues leaked rejected reference %q: %s", rejected, encoded)
		}
	}
}
