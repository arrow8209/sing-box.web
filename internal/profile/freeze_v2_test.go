package profile

import (
	"encoding/json"
	"testing"

	"singbox-web/internal/configref"
)

type acceptedFreeze struct{}

func (acceptedFreeze) VerifyFreeze(string, uint64, FreezeOperation) error { return nil }

func TestVersionTwoCopyPreservesMixedInboundOrder(t *testing.T) {
	refs := configref.Persisted{Version: 2, Outbounds: []configref.OutboundInput{}, Inbounds: []configref.InboundBinding{
		{Tag: "A", ResourceID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Position: 0},
		{Tag: "B", ResourceID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Position: 2},
	}}
	draft := Draft{ProfileID: "cccccccccccccccccccccccccccccccc", Revision: 4, SchemaVersion: 2, References: &refs, Buffers: map[string]string{"inbounds": `[{"type":"mixed","tag":"X","future":9007199254740993}]`, "route": `{"rules":[]}`}, Config: map[string]json.RawMessage{}, EditorState: map[string]json.RawMessage{}}
	op := FreezeOperation{Action: "copy", Module: "inbounds", Tag: "A", NewTag: "A-copy", Text: `{"type":"mixed","tag":"A","future":9007199254740995}`, InboundStateText: `{"version":1,"managedSniffRules":[]}`}
	copied, err := applyFreezeV2(draft, op, acceptedFreeze{}, savedCopy{})
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(copied.Buffers["inbounds"]), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || freezeString(items[0]["tag"]) != "A-copy" || freezeString(items[1]["tag"]) != "X" || copied.References.Inbounds[1].Position != 3 {
		t.Fatal(copied)
	}
	if len(draft.References.Inbounds) != 2 || draft.References.Inbounds[1].Position != 2 {
		t.Fatal("input mutated")
	}
}
