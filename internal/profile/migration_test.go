package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMigrationPreservesIndependentRawJSONAndBacksUpDraft(t *testing.T) {
	s, layout := newTestService(t)
	ctx := context.Background()
	meta, draft, err := s.Create(ctx, CreateInput{Name: "迁移样例"})
	if err != nil {
		t.Fatal(err)
	}
	const sid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const iid = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	inputs := map[string]string{
		"outbounds": `[{"type":"selector","tag":"组","outbounds":["old"],"default":"old"},{"type":"socks","tag":"old","server":"old.example.test"},{"type":"direct","tag":"独立","future":9007199254740993}]`,
		"inbounds":  `[{"type":"mixed","tag":"local","listen_port":19090},{"type":"mixed","tag":"standalone","future":9007199254740995}]`,
		"route":     `{"rules":[{"action":"sniff","inbound":"local","timeout":"300ms"},{"action":"route","outbound":"独立","future":9007199254740997}]}`,
	}
	state := map[string]json.RawMessage{
		"outbounds": json.RawMessage(`{"version":1,"groups":[{"id":"g","tag":"组","type":"selector","hidden":true,"include":"","exclude":"","icon":"star","references":[{"kind":"subscription","subscriptionId":"` + sid + `","tag":"source","resolvedTags":["old"]}],"runtimeTags":["old"]}],"generatedTags":["old"]}`),
		"inbounds":  json.RawMessage(`{"version":1,"managedSniffRules":[{"tag":"local","rule":{"inbound":"local","action":"sniff","timeout":"300ms"}}],"resourceBindings":[{"tag":"local","resourceId":"` + iid + `","sourceTag":"source"}]}`),
	}
	old, _, err := s.SaveDraft(ctx, meta.ID, RevisionETag(draft.Revision), DraftInput{Buffers: inputs, EditorState: state, Bindings: []RuleBinding{}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.store.draftPath(meta.ID))
	if err != nil {
		t.Fatal(err)
	}
	read, _, err := s.GetDraft(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterRead, err := os.ReadFile(s.store.draftPath(meta.ID))
	if err != nil {
		t.Fatal(err)
	}
	if read.SchemaVersion != 0 || string(before) != string(afterRead) {
		t.Fatal("opening legacy draft wrote a migration")
	}
	newDraft, _, err := s.SaveDraft(ctx, meta.ID, RevisionETag(old.Revision), DraftInput{SchemaVersion: 2, Buffers: old.Buffers, EditorState: old.EditorState, Bindings: old.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	if newDraft.SchemaVersion != 2 || newDraft.References == nil || len(newDraft.References.Outbounds) != 1 || len(newDraft.References.Inbounds) != 1 {
		t.Fatal(newDraft.References)
	}
	if strings.Contains(newDraft.Buffers["outbounds"], `"tag":"old"`) || strings.Contains(newDraft.Buffers["outbounds"], `"outbounds":["old"]`) || !strings.Contains(newDraft.Buffers["outbounds"], "9007199254740993") {
		t.Fatal(newDraft.Buffers["outbounds"])
	}
	if strings.Contains(newDraft.Buffers["inbounds"], `"tag":"local"`) || !strings.Contains(newDraft.Buffers["inbounds"], "9007199254740995") {
		t.Fatal(newDraft.Buffers["inbounds"])
	}
	if strings.Contains(newDraft.Buffers["route"], `"action":"sniff"`) || !strings.Contains(newDraft.Buffers["route"], "9007199254740997") {
		t.Fatal(newDraft.Buffers["route"])
	}
	if strings.Contains(string(newDraft.EditorState["outbounds"]), "subscriptionId") || len(newDraft.EditorState["inbounds"]) > 0 {
		t.Fatal(newDraft.EditorState)
	}
	backup := filepath.Join(layout.DraftsDir, "migration-backups", meta.ID+"-rev2-v1.json")
	archived, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archived), "9007199254740993") || strings.Contains(string(archived), `"schemaVersion": 2`) {
		t.Fatal("legacy backup invalid")
	}
	_, _, err = s.SaveDraft(ctx, meta.ID, RevisionETag(newDraft.Revision), DraftInput{Buffers: newDraft.Buffers, EditorState: newDraft.EditorState, Bindings: []RuleBinding{}})
	if !errors.Is(err, ErrSchemaVersionUnsupported) {
		t.Fatalf("old client write = %v", err)
	}
	current, _, err := s.GetDraft(ctx, meta.ID)
	if err != nil || current.Revision != newDraft.Revision {
		t.Fatalf("old client changed draft: %+v %v", current, err)
	}
	copyMeta, copyDraft, err := s.Duplicate(ctx, meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if copyMeta.ID == meta.ID || copyDraft.SchemaVersion != 2 || copyDraft.References == nil || copyDraft.Revision != 1 {
		t.Fatal(copyDraft)
	}
	// Re-saving a migrated draft cannot expand generated copies into input.
	again, _, err := s.SaveDraft(ctx, meta.ID, RevisionETag(newDraft.Revision), DraftInput{SchemaVersion: 2, Buffers: newDraft.Buffers, EditorState: newDraft.EditorState, References: newDraft.References, Bindings: []RuleBinding{}})
	if err != nil {
		t.Fatal(err)
	}
	if again.Buffers["outbounds"] != newDraft.Buffers["outbounds"] || again.Buffers["inbounds"] != newDraft.Buffers["inbounds"] {
		t.Fatal("migration drifted on repeat save")
	}
}

func TestDamagedLegacyMetadataCannotBeSilentlyMigrated(t *testing.T) {
	s, _ := newTestService(t)
	ctx := context.Background()
	meta, draft, err := s.Create(ctx, CreateInput{Name: "损坏元数据"})
	if err != nil {
		t.Fatal(err)
	}
	bad := map[string]json.RawMessage{"inbounds": json.RawMessage(`{"version":9,"resourceBindings":[]}`)}
	old, _, err := s.SaveDraft(ctx, meta.ID, RevisionETag(draft.Revision), DraftInput{Buffers: map[string]string{"inbounds": `[]`}, EditorState: bad, Bindings: []RuleBinding{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveDraft(ctx, meta.ID, RevisionETag(old.Revision), DraftInput{SchemaVersion: 2, Buffers: old.Buffers, EditorState: old.EditorState, Bindings: old.Bindings}); err == nil {
		t.Fatal("damaged metadata migrated")
	}
	current, _, err := s.GetDraft(ctx, meta.ID)
	if err != nil || current.Revision != old.Revision {
		t.Fatal("failed migration wrote data")
	}
}

func TestMigrationKeepsIndependentSniffAndRoutePositions(t *testing.T) {
	const linked = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	old := Draft{ProfileID: strings.Repeat("c", 32), Revision: 1, Config: map[string]json.RawMessage{}, Buffers: map[string]string{
		"inbounds": `[{"type":"mixed","tag":"linked"},{"type":"mixed","tag":"independent"}]`,
		"route":    `{"rules":[{"inbound":"linked","action":"sniff","timeout":"300ms"},{"inbound":"independent","action":"sniff","timeout":"500ms"},{"action":"route","outbound":"direct"}]}`,
	}, EditorState: map[string]json.RawMessage{
		"inbounds": json.RawMessage(`{"version":1,"managedSniffRules":[{"tag":"linked","rule":{"inbound":"linked","action":"sniff","timeout":"300ms"}},{"tag":"independent","rule":{"inbound":"independent","action":"sniff","timeout":"500ms"}}],"resourceBindings":[{"tag":"linked","resourceId":"` + linked + `","sourceTag":"source"}]}`),
		"route":    json.RawMessage(`{"version":1,"insertionPosition":3,"disabledRules":[{"position":0,"rule":{"action":"reject"}}]}`),
	}}
	next, err := migrateLegacyDraft(old)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(next.Buffers["route"], `"inbound":"linked"`) || !strings.Contains(next.Buffers["route"], `"inbound":"independent"`) {
		t.Fatal(next.Buffers["route"])
	}
	if !strings.Contains(string(next.EditorState["route"]), `"insertionPosition":2`) {
		t.Fatal(string(next.EditorState["route"]))
	}
	if next.References == nil || len(next.References.Inbounds) != 1 {
		t.Fatal(next.References)
	}
}
