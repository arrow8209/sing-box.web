package configgen

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"singbox-web/internal/apierror"
	"singbox-web/internal/configref"
	"singbox-web/internal/inbounds"
	"singbox-web/internal/profile"
	"singbox-web/internal/subscriptions"
)

type referenceFixture struct {
	Name          string                       `json:"name"`
	Draft         profile.Draft                `json:"draft"`
	Subscriptions []subscriptions.Subscription `json:"subscriptions"`
	Inbounds      []inbounds.Inbound           `json:"inbounds"`
	Expected      map[string]json.RawMessage   `json:"expected"`
	Code          string                       `json:"code"`
	FieldPath     string                       `json:"fieldPath"`
}

func referenceFixtures(t *testing.T) []referenceFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/references/legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []referenceFixture
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}
func fixtureSnapshot(f referenceFixture) ReferenceSnapshot {
	s := ReferenceSnapshot{Inbounds: map[string]inbounds.Inbound{}, Subscriptions: map[string]subscriptions.Subscription{}}
	for _, v := range f.Inbounds {
		s.Inbounds[v.ID] = v
	}
	for _, v := range f.Subscriptions {
		s.Subscriptions[v.ID] = v
	}
	return s
}
func assertReferenceError(t *testing.T, err error, code, path string) {
	t.Helper()
	var e *apierror.HTTPError
	if !errors.As(err, &e) || e.Public.Code != code || e.Public.Stage != configref.Stage || (path != "" && e.Public.FieldPath != path) {
		t.Fatalf("error = %#v, want %s at %s", err, code, path)
	}
}

func TestReferenceCompatibilityFixtures(t *testing.T) {
	for _, f := range referenceFixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			before := rawValue(f.Draft)
			s := fixtureSnapshot(f)
			sources := rawValue(s)
			result, err := ResolveReferences(f.Draft, s)
			if string(before) != string(rawValue(f.Draft)) || string(sources) != string(rawValue(s)) {
				t.Fatal("mutated inputs")
			}
			if f.Code != "" {
				assertReferenceError(t, err, f.Code, f.FieldPath)
				if result.Draft.Config != nil {
					t.Fatal("partial result on failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range f.Expected {
				if !rawEqual(result.Draft.Config[key], want) {
					t.Fatalf("%s = %s, want %s", key, result.Draft.Config[key], want)
				}
				if result.Draft.Buffers[key] != string(result.Draft.Config[key]) {
					t.Fatalf("stale buffer %s", key)
				}
			}
			again, err := ResolveReferences(result.Draft, s)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result, again) {
				t.Fatalf("not idempotent\n%s\n%s", rawValue(result), rawValue(again))
			}
		})
	}
}

func TestReferenceFilterCompatibility(t *testing.T) {
	raw, err := os.ReadFile("testdata/references/filters.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Pattern, Tag, Code string
		Match              bool
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Pattern+"/"+c.Tag, func(t *testing.T) {
			m, e := referenceFilter(c.Pattern, "filter")
			var got bool
			if e == nil {
				got, e = m(c.Tag)
			}
			if c.Code != "" {
				assertReferenceError(t, e, c.Code, "filter")
				return
			}
			if e != nil || got != c.Match {
				t.Fatalf("match = %v, %v, want %v", got, e, c.Match)
			}
		})
	}
}

func TestReferenceSharedAliasesAndSourceOrder(t *testing.T) {
	f := referenceFixtures(t)[0]
	state := configref.Outbounds{}
	_ = json.Unmarshal(f.Draft.EditorState["outbounds"], &state)
	state.Groups = append(state.Groups, state.Groups[0])
	state.Groups[1].ID = "second"
	state.Groups[1].Tag = "备用"
	var items []json.RawMessage
	_ = json.Unmarshal(f.Draft.Config["outbounds"], &items)
	items = append(items, json.RawMessage(`{"type":"selector","tag":"备用","outbounds":["old"]}`), json.RawMessage(`{"type":"direct","tag":"new"}`))
	f.Draft.EditorState["outbounds"] = rawValue(state)
	setModule(&f.Draft, "outbounds", rawValue(items))
	s := fixtureSnapshot(f)
	sub := f.Subscriptions[0]
	sub.Outbounds = append(sub.Outbounds, json.RawMessage(`{"type":"socks","tag":"next","detour":"new","default":"new","outbounds":["new"]}`))
	s.Subscriptions[sub.ID] = sub
	r, err := ResolveReferences(f.Draft, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range r.Model.Outbounds.Groups {
		if !reflect.DeepEqual(g.RuntimeTags, []string{"new-2", "next"}) {
			t.Fatal(g.RuntimeTags)
		}
	}
	if !reflect.DeepEqual(r.Model.Outbounds.GeneratedTags, []string{"new-2", "next"}) {
		t.Fatal(r.Model.Outbounds.GeneratedTags)
	}
	if !strings.Contains(string(r.Draft.Config["outbounds"]), `"detour":"new-2"`) {
		t.Fatal(string(r.Draft.Config["outbounds"]))
	}
	sub.Outbounds[0], sub.Outbounds[1] = sub.Outbounds[1], sub.Outbounds[0]
	s.Subscriptions[sub.ID] = sub
	r, err = ResolveReferences(f.Draft, s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Model.Outbounds.Groups[0].RuntimeTags, []string{"next", "new-2"}) {
		t.Fatal(r.Model.Outbounds.Groups)
	}
}

func TestReferenceDefaultRemovedAndNoPartialOutput(t *testing.T) {
	f := referenceFixtures(t)[0]
	text := strings.Replace(f.Draft.Buffers["outbounds"], `"type":"selector"`, `"default":"old","type":"selector"`, 1)
	f.Draft.Buffers["outbounds"] = text
	r, e := ResolveReferences(f.Draft, fixtureSnapshot(f))
	assertReferenceError(t, e, "reference_conflict", "outbounds[0].default")
	if r.Draft.Config != nil {
		t.Fatal("partial candidate")
	}
}

func TestReferenceSniffOwnershipAndPositions(t *testing.T) {
	f := referenceFixtures(t)[7]
	s := fixtureSnapshot(f)
	r, e := ResolveReferences(f.Draft, s)
	if e != nil {
		t.Fatal(e)
	}
	// Preserve a manual rule before the managed rule; update in place.
	route := object(r.Draft.Config["route"])
	var rules []json.RawMessage
	_ = json.Unmarshal(route["rules"], &rules)
	manual := json.RawMessage(`{"inbound":"manual","action":"sniff","timeout":"1s","future":9007199254740999}`)
	rules = append([]json.RawMessage{append(json.RawMessage{}, manual...)}, rules...)
	route["rules"] = rawValue(rules)
	setModule(&r.Draft, "route", rawValue(route))
	source := f.Inbounds[0]
	source.Config = json.RawMessage(strings.Replace(string(source.Config), "750ms", "900ms", 1))
	s.Inbounds[source.ID] = source
	r, e = ResolveReferences(r.Draft, s)
	if e != nil {
		t.Fatal(e)
	}
	route = object(r.Draft.Config["route"])
	_ = json.Unmarshal(route["rules"], &rules)
	if !rawEqual(rules[0], manual) || rawString(object(rules[1])["timeout"]) != "900ms" {
		t.Fatal(string(route["rules"]))
	}
	// Another source inserts at the front without reordering existing rules.
	other := source
	other.ID = strings.Repeat("d", 32)
	s.Inbounds[other.ID] = other
	var items []json.RawMessage
	_ = json.Unmarshal(r.Draft.Config["inbounds"], &items)
	items = append(items, json.RawMessage(`{"type":"mixed","tag":"second"}`))
	setModule(&r.Draft, "inbounds", rawValue(items))
	state := r.Model.Inbounds
	state.ResourceBindings = append(state.ResourceBindings, configref.InboundBinding{Tag: "second", ResourceID: other.ID, SourceTag: "source"})
	r.Draft.EditorState["inbounds"] = rawValue(state)
	r.Draft.EditorState["route"] = json.RawMessage(`{"version":1,"insertionPosition":1,"disabledRules":[{"position":0,"rule":{"action":"reject"}}]}`)
	r, e = ResolveReferences(r.Draft, s)
	if e != nil {
		t.Fatal(e)
	}
	route = object(r.Draft.Config["route"])
	_ = json.Unmarshal(route["rules"], &rules)
	if rawString(object(rules[0])["inbound"]) != "second" || !rawEqual(rules[1], manual) || rawString(object(rules[2])["inbound"]) != "local" {
		t.Fatal(string(route["rules"]))
	}
	if !strings.Contains(string(r.Draft.EditorState["route"]), `"insertionPosition":2`) {
		t.Fatal(string(r.Draft.EditorState["route"]))
	}
	// Disabling a source removes only its own snapshot.
	changed := object(source.Config)
	changed["sniff"] = rawValue(false)
	source.Config = rawValue(changed)
	s.Inbounds[source.ID] = source
	r, e = ResolveReferences(r.Draft, s)
	if e != nil {
		t.Fatal(e)
	}
	route = object(r.Draft.Config["route"])
	_ = json.Unmarshal(route["rules"], &rules)
	if len(rules) != 3 || !rawEqual(rules[1], manual) {
		t.Fatal(string(route["rules"]))
	}
}

func TestReferenceCustomAndDeletedInbound(t *testing.T) {
	f := referenceFixtures(t)[7]
	s := fixtureSnapshot(f)
	source := f.Inbounds[0]
	source.Mode = inbounds.ModeCustom
	s.Inbounds[source.ID] = source
	r, e := ResolveReferences(f.Draft, s)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(r.Draft.Config["inbounds"]), `"sniff":true`) || !rawEqual(r.Draft.Config["route"], f.Draft.Config["route"]) {
		t.Fatal(string(r.Draft.Config["inbounds"]))
	}
	setModule(&f.Draft, "inbounds", json.RawMessage(`[]`))
	r, e = ResolveReferences(f.Draft, ReferenceSnapshot{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Model.Inbounds.ResourceBindings) != 0 || len(r.ResourceDigests) != 0 {
		t.Fatal(r)
	}
}

func TestReferenceDigestsCoverGenerationInputs(t *testing.T) {
	f := referenceFixtures(t)[7]
	a := f.Inbounds[0]
	b := a
	b.Mode = inbounds.ModeCustom
	if InboundDigest(a) == InboundDigest(b) {
		t.Fatal("mode omitted")
	}
	f = referenceFixtures(t)[0]
	sub := f.Subscriptions[0]
	other := sub
	other.Name = "renamed"
	if SubscriptionDigest(sub) != SubscriptionDigest(other) {
		t.Fatal("display metadata affects digest")
	}
	other.Outbounds = append(append([]json.RawMessage{}, sub.Outbounds...), json.RawMessage(`{"tag":"extra","type":"direct"}`))
	if SubscriptionDigest(sub) == SubscriptionDigest(other) {
		t.Fatal("nodes omitted")
	}
}
