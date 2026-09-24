package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/templates"
)

type testBindingSource struct{ bindings []RuleBinding }

func (s testBindingSource) DefaultBindings(context.Context) ([]RuleBinding, error) {
	return append([]RuleBinding(nil), s.bindings...), nil
}

func testClock() time.Time { return time.Unix(1, 0).UTC() }

func newTestService(t *testing.T) (*Service, paths.Layout) {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	factory, err := templates.NewFactory(bytes.NewReader(bytes.Repeat([]byte{0x11}, 128)))
	if err != nil {
		t.Fatal(err)
	}
	ids := append(bytes.Repeat([]byte{0x22}, 16), bytes.Repeat([]byte{0x33}, 16)...)
	ids = append(ids, bytes.Repeat([]byte{0x44}, 16)...)
	service, err := Open(layout, factory, testBindingSource{bindings: []RuleBinding{{ResourceID: "resource-a", Tag: "geosite-cn", Enabled: true}}}, testClock, bytes.NewReader(ids))
	if err != nil {
		t.Fatal(err)
	}
	return service, layout
}

func TestCreateProfileUsesTemplateAndNoProfileDirectory(t *testing.T) {
	service, layout := newTestService(t)
	meta, draft, err := service.Create(context.Background(), CreateInput{Name: "家庭出口"})
	if err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" || meta.Name != "家庭出口" || draft.Revision != 1 {
		t.Fatalf("meta=%+v draft=%+v", meta, draft)
	}
	if _, ok := draft.Config["experimental"]; !ok {
		t.Fatal("template not loaded")
	}
	if _, err := os.Stat(filepath.Join(layout.ConfigDir, meta.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("profile config directory exists: %v", err)
	}
}

func TestDuplicateProfileDoesNotInheritApplicationState(t *testing.T) {
	service, _ := newTestService(t)
	meta, draft, err := service.Create(context.Background(), CreateInput{Name: "家庭出口"})
	if err != nil {
		t.Fatal(err)
	}
	appliedAt := time.Unix(2, 0).UTC()
	if err := service.MarkApplied(context.Background(), AppliedResult{
		ProfileID: meta.ID,
		Revision:  draft.Revision,
		Digest:    "source-digest",
		AppliedAt: appliedAt,
		Validation: ValidationSummary{
			Valid: true, Message: "配置检查通过", CheckedAt: appliedAt,
		},
	}); err != nil {
		t.Fatal(err)
	}

	copiedMeta, copiedDraft, err := service.Duplicate(context.Background(), meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if copiedMeta.Name != "家庭出口 副本" || copiedMeta.DraftRevision != 1 || !copiedMeta.NeedsApply {
		t.Fatalf("copied meta=%+v", copiedMeta)
	}
	if copiedMeta.AppliedRevision != nil || copiedMeta.AppliedDigest != "" || copiedMeta.AppliedAt != nil || copiedMeta.ConfigCurrent || copiedMeta.LastValidation != nil {
		t.Fatalf("application state leaked into copy: %+v", copiedMeta)
	}
	if copiedDraft.ProfileID != copiedMeta.ID || copiedDraft.Revision != 1 || copiedDraft.Buffers["route"] != draft.Buffers["route"] {
		t.Fatalf("copied draft=%+v", copiedDraft)
	}
}

func TestOnlyMostRecentlyAppliedProfileOwnsCurrentConfig(t *testing.T) {
	service, _ := newTestService(t)
	first, firstDraft, err := service.Create(context.Background(), CreateInput{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	second, secondDraft, err := service.Create(context.Background(), CreateInput{Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	apply := func(id string, revision uint64, at time.Time) {
		t.Helper()
		if err := service.MarkApplied(context.Background(), AppliedResult{
			ProfileID: id, Revision: revision, Digest: id, AppliedAt: at,
			Validation: ValidationSummary{Valid: true, CheckedAt: at},
		}); err != nil {
			t.Fatal(err)
		}
	}
	apply(first.ID, firstDraft.Revision, time.Unix(2, 0))
	apply(second.ID, secondDraft.Revision, time.Unix(3, 0))

	metas, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	current := ""
	for _, meta := range metas {
		if meta.ConfigCurrent {
			if current != "" {
				t.Fatalf("multiple current config profiles: %+v", metas)
			}
			current = meta.ID
		}
	}
	if current != second.ID {
		t.Fatalf("current=%q want=%q metas=%+v", current, second.ID, metas)
	}
}

func TestDraftRoundTripPreservesUnknownOutboundFields(t *testing.T) {
	service, _ := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "手工节点"})
	if err != nil {
		t.Fatal(err)
	}
	outbound := json.RawMessage(`[{"type":"future-protocol","tag":"node-a","future":{"nested":[1,"x",true]}}]`)
	saved, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{Buffers: map[string]string{"outbounds": string(outbound)}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved.Config["outbounds"], outbound) {
		t.Fatalf("outbounds changed: %s", saved.Config["outbounds"])
	}
}

func TestDraftEditorStatePersistsOutsideGeneratedConfig(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "出站编辑状态"})
	if err != nil {
		t.Fatal(err)
	}
	state := json.RawMessage(`{"version":1,"groups":[{"id":"group-a","tag":"节点选择","type":"selector","hidden":true,"include":"香港","exclude":"","icon":"","references":[],"runtimeTags":[]}],"generatedTags":[]}`)
	saved, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{
		Buffers:     map[string]string{},
		EditorState: map[string]json.RawMessage{"outbounds": state},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved.EditorState["outbounds"], state) {
		t.Fatalf("editor state changed: %s", saved.EditorState["outbounds"])
	}
	if _, exists := saved.Config["editorState"]; exists {
		t.Fatal("editor state leaked into generated config")
	}

	reopened, err := Open(layout, service.templates, service.defaultBindings, testClock, bytes.NewReader(bytes.Repeat([]byte{0x44}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	draft, _, err := reopened.GetDraft(context.Background(), meta.ID)
	var compact bytes.Buffer
	if err == nil {
		err = json.Compact(&compact, draft.EditorState["outbounds"])
	}
	if err != nil || !bytes.Equal(compact.Bytes(), state) {
		t.Fatalf("editor state was not persisted, err=%v state=%s", err, draft.EditorState["outbounds"])
	}
	if draft.Bindings == nil {
		t.Fatal("empty bindings were returned as null")
	}
}

func TestDraftRoundTripPreservesUnknownTopLevelModule(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "未来模块"})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"futureInteger":9007199254740993,"nested":["x",true]}`
	saved, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{Buffers: map[string]string{"future_module": raw}})
	if err != nil {
		t.Fatal(err)
	}
	if string(saved.Config["future_module"]) != raw || saved.Buffers["future_module"] != raw {
		t.Fatalf("saved=%+v", saved)
	}
	reopened, err := Open(layout, service.templates, service.defaultBindings, testClock, bytes.NewReader(bytes.Repeat([]byte{0x44}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	draft, _, err := reopened.GetDraft(context.Background(), meta.ID)
	if err != nil || !bytes.Contains(draft.Config["future_module"], []byte("9007199254740993")) || draft.Buffers["future_module"] != raw {
		t.Fatalf("unknown module was not preserved, err=%v", err)
	}
}

func TestSaveDraftMergesPartialBuffersAndPreservesInvalidBuffer(t *testing.T) {
	service, _ := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "局部缓冲"})
	if err != nil {
		t.Fatal(err)
	}
	original, _, err := service.GetDraft(context.Background(), meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{Buffers: map[string]string{"outbounds": "[{"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved.Config["outbounds"], original.Config["outbounds"]) {
		t.Fatalf("invalid buffer overwrote valid config: %s", saved.Config["outbounds"])
	}
	if saved.Buffers["outbounds"] != "[{" {
		t.Fatalf("invalid buffer was not retained: %q", saved.Buffers["outbounds"])
	}
	if saved.Buffers["dns"] != original.Buffers["dns"] {
		t.Fatal("unsubmitted dns buffer was discarded")
	}
}

func TestCorruptSingleDraftQuarantinesOnlyThatProfile(t *testing.T) {
	service, layout := newTestService(t)
	first, _, err := service.Create(context.Background(), CreateInput{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := service.Create(context.Background(), CreateInput{Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.DraftsDir, first.ID+".json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	profiles, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]Meta{}
	for _, item := range profiles {
		found[item.ID] = item
	}
	if found[first.ID].QuarantinedError == "" {
		t.Fatalf("profile %s was not quarantined: %+v", first.ID, found[first.ID])
	}
	if found[second.ID].QuarantinedError != "" {
		t.Fatalf("profile %s unexpectedly quarantined: %+v", second.ID, found[second.ID])
	}
}

func TestSaveDraftRejectsStaleRevision(t *testing.T) {
	service, _ := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "冲突"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{}); err != nil {
		t.Fatal(err)
	}
	_, etag, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{})
	var conflict RevisionConflict
	if !errors.As(err, &conflict) || etag != `"rev-2"` || conflict.LatestRevision != 2 {
		t.Fatalf("err=%v etag=%q conflict=%+v", err, etag, conflict)
	}
}

func TestConcurrentSaveAllowsOnlyOneMatchingRevision(t *testing.T) {
	service, _ := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "并发"})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _, saveErr := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{})
			results <- saveErr
		}()
	}
	group.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for result := range results {
		if result == nil {
			successes++
			continue
		}
		var conflict RevisionConflict
		if errors.As(result, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected save error: %v", result)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestMarkResourcesChangedIgnoresDisabledBindings(t *testing.T) {
	service, _ := newTestService(t)
	meta, draft, err := service.Create(context.Background(), CreateInput{Name: "禁用资源"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.MarkApplied(context.Background(), AppliedResult{ProfileID: meta.ID, Revision: draft.Revision, AppliedAt: testClock()}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{Bindings: []RuleBinding{{ResourceID: "resource-a", Tag: "geosite-cn", Enabled: false}}}); err != nil {
		t.Fatal(err)
	}
	if err := service.MarkApplied(context.Background(), AppliedResult{ProfileID: meta.ID, Revision: 2, AppliedAt: testClock()}); err != nil {
		t.Fatal(err)
	}
	if err := service.MarkResourcesChanged(context.Background(), []string{"resource-a"}); err != nil {
		t.Fatal(err)
	}
	profiles, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if profiles[0].NeedsApply {
		t.Fatalf("disabled binding changed needsApply: %+v", profiles[0])
	}
}

func TestResourceDigestsMergeAndValidationExpiresWithoutDraftRevision(t *testing.T) {
	service, layout := newTestService(t)
	ctx := context.Background()
	meta, draft, err := service.Create(ctx, CreateInput{Name: "引用档案"})
	if err != nil {
		t.Fatal(err)
	}
	other, otherDraft, err := service.Create(ctx, CreateInput{Name: "独立档案"})
	if err != nil {
		t.Fatal(err)
	}
	const sid = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	config := `[{"type":"selector","tag":"组","outbounds":["node"]},{"type":"socks","tag":"node","server":"old"}]`
	state := `{"version":1,"groups":[{"id":"g","tag":"组","type":"selector","hidden":false,"include":"","exclude":"","icon":"","references":[{"kind":"subscription","subscriptionId":"` + sid + `","tag":"源","resolvedTags":["node"]}],"runtimeTags":["node"]}],"generatedTags":["node"]}`
	draft, _, err = service.SaveDraft(ctx, meta.ID, RevisionETag(draft.Revision), DraftInput{Buffers: map[string]string{"outbounds": config}, EditorState: map[string]json.RawMessage{"outbounds": json.RawMessage(state)}})
	if err != nil {
		t.Fatal(err)
	}
	otherDraft, _, err = service.SaveDraft(ctx, other.ID, RevisionETag(otherDraft.Revision), DraftInput{Buffers: map[string]string{"outbounds": `[]`}})
	if err != nil {
		t.Fatal(err)
	}
	initial := map[string]string{"subscription:" + sid: "v1", "rule:resource-a": "rule-v1"}
	if err := service.ReconcileAllResourceDigests(ctx, initial); err != nil {
		t.Fatal(err)
	}
	apply := func(id string, revision uint64, digests map[string]string) {
		t.Helper()
		if err := service.MarkApplied(ctx, AppliedResult{ProfileID: id, Revision: revision, Digest: "config-v1", ResourceDigests: digests, AppliedAt: testClock(), Validation: ValidationSummary{Valid: true, Stage: "core_check", CheckedAt: testClock()}}); err != nil {
			t.Fatal(err)
		}
	}
	apply(meta.ID, draft.Revision, map[string]string{"subscription:" + sid: "v1"})
	apply(other.ID, otherDraft.Revision, map[string]string{})
	if err := service.RecordValidation(ctx, meta.ID, draft.Revision, ValidationSummary{Valid: true, Stage: "core_check", ConfigDigest: "config-v1", ResourceDigests: map[string]string{"subscription:" + sid: "v1"}, CheckedAt: testClock()}); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileTypedResourceDigests(ctx, "subscription", map[string]string{"subscription:" + sid: "v2"}); err != nil {
		t.Fatal(err)
	}
	metas, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range metas {
		if m.ID == meta.ID {
			if !m.NeedsApply || m.LastValidation == nil || !m.LastValidation.Stale || m.DraftRevision != draft.Revision {
				t.Fatalf("stale linked profile: %+v", m)
			}
		} else if m.ID == other.ID && m.NeedsApply {
			t.Fatalf("unrelated profile changed: %+v", m)
		}
	}
	if service.resourceDigests["rule:resource-a"] != "rule-v1" {
		t.Fatal("subscription update erased rule digest")
	}
	if err := service.ReconcileResourceDigests(ctx, map[string]string{"resource-a": "rule-v2"}); err != nil {
		t.Fatal(err)
	}
	if service.resourceDigests["subscription:"+sid] != "v2" {
		t.Fatal("rule update erased subscription digest")
	}
	// Reopening and reconciling all source types preserves the stale conclusion.
	factory, err := templates.NewFactory(bytes.NewReader(bytes.Repeat([]byte{0x11}, 128)))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(layout, factory, testBindingSource{}, testClock, bytes.NewReader(bytes.Repeat([]byte{0x55}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.ReconcileAllResourceDigests(ctx, map[string]string{"subscription:" + sid: "v2", "rule:resource-a": "rule-v2"}); err != nil {
		t.Fatal(err)
	}
	metas, err = reopened.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range metas {
		if m.ID == meta.ID && (!m.NeedsApply || m.LastValidation == nil || !m.LastValidation.Stale) {
			t.Fatalf("restart lost stale state: %+v", m)
		}
	}
}

func TestOlderApplyCannotClearNewSourceChange(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()
	meta, draft, err := service.Create(ctx, CreateInput{Name: "并发引用"})
	if err != nil {
		t.Fatal(err)
	}
	const id = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	draft, _, err = service.SaveDraft(ctx, meta.ID, RevisionETag(draft.Revision), DraftInput{Buffers: map[string]string{"inbounds": `[{"type":"mixed","tag":"local"}]`}, EditorState: map[string]json.RawMessage{"inbounds": json.RawMessage(`{"version":1,"managedSniffRules":[],"resourceBindings":[{"tag":"local","resourceId":"` + id + `","sourceTag":"source"}]}`)}})
	if err != nil {
		t.Fatal(err)
	}
	key := "inbound:" + id
	if err := service.ReconcileAllResourceDigests(ctx, map[string]string{key: "new"}); err != nil {
		t.Fatal(err)
	}
	if err := service.MarkApplied(ctx, AppliedResult{ProfileID: meta.ID, Revision: draft.Revision, Digest: "old-config", ResourceDigests: map[string]string{key: "old"}, AppliedAt: testClock(), Validation: ValidationSummary{Valid: true, CheckedAt: testClock()}}); err != nil {
		t.Fatal(err)
	}
	metas, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !metas[0].NeedsApply || metas[0].LastValidation == nil || !metas[0].LastValidation.Stale {
		t.Fatalf("older apply cleared change: %+v", metas[0])
	}
}

func TestServiceReopensDraftAndQuarantineState(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "重启"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(layout, service.templates, service.defaultBindings, testClock, bytes.NewReader(bytes.Repeat([]byte{0x33}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	draft, etag, err := reopened.GetDraft(context.Background(), meta.ID)
	if err != nil || draft.Revision != 2 || etag != `"rev-2"` {
		t.Fatalf("draft=%+v etag=%q err=%v", draft, etag, err)
	}
}

func TestRecoverDeletingDraftRestoresExistingProfile(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "恢复删除"})
	if err != nil {
		t.Fatal(err)
	}
	draftPath := filepath.Join(layout.DraftsDir, meta.ID+".json")
	stagedPath := draftPath + ".deleting"
	if err := os.Rename(draftPath, stagedPath); err != nil {
		t.Fatal(err)
	}
	if err := RecoverDeletingDrafts(layout); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("draft was not restored: %v", err)
	}
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged draft remains: %v", err)
	}
}

func TestRecoverDeletingDraftCleansDeletedProfile(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "清理删除"})
	if err != nil {
		t.Fatal(err)
	}
	draftPath := filepath.Join(layout.DraftsDir, meta.ID+".json")
	stagedPath := draftPath + ".deleting"
	if err := os.Rename(draftPath, stagedPath); err != nil {
		t.Fatal(err)
	}
	if err := service.store.writeMetas([]Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := RecoverDeletingDrafts(layout); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted profile draft remains: %v", err)
	}
}

func TestDeleteReturnsSuccessAfterMetadataCommitWhenCleanupFails(t *testing.T) {
	service, layout := newTestService(t)
	service.SetDeleteGuard(allowDeleteGuard{})
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "一致删除"})
	if err != nil {
		t.Fatal(err)
	}
	service.store.removeFile = func(string) error { return errors.New("injected cleanup failure") }
	if err := service.Delete(context.Background(), meta.ID, DeleteInput{ConfirmName: meta.Name}); err != nil {
		t.Fatalf("committed delete reported failure: %v", err)
	}
	metas, err := service.List(context.Background())
	if err != nil || len(metas) != 0 {
		t.Fatalf("metas=%+v err=%v", metas, err)
	}
	stagedPath := filepath.Join(layout.DraftsDir, meta.ID+".json.deleting")
	if _, err := os.Stat(stagedPath); err != nil {
		t.Fatalf("expected staged cleanup recovery file: %v", err)
	}
	if err := RecoverDeletingDrafts(layout); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("startup recovery did not clean staged draft: %v", err)
	}
}
