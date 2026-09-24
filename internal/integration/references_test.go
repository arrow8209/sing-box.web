package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/application"
	"singbox-web/internal/configgen"
	"singbox-web/internal/inbounds"
	"singbox-web/internal/profile"
	"singbox-web/internal/subscriptions"
)

type referenceLibrary struct {
	subscription subscriptions.Subscription
	inbound      inbounds.Inbound
	reads        map[string]int
}
type inboundLibrary struct{ library *referenceLibrary }
type subscriptionLibrary struct{ library *referenceLibrary }

func (s inboundLibrary) Get(_ context.Context, id string) (inbounds.Inbound, error) {
	s.library.reads["inbound:"+id]++
	return s.library.inbound, nil
}
func (s subscriptionLibrary) Get(_ context.Context, id string) (subscriptions.Subscription, error) {
	s.library.reads["subscription:"+id]++
	return s.library.subscription, nil
}

func saveReferenceFixture(t *testing.T, env *integrationEnvironment) (string, uint64, *referenceLibrary) {
	t.Helper()
	raw, err := os.ReadFile("../configgen/testdata/references/legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Draft         profile.Draft
		Subscriptions []subscriptions.Subscription
		Inbounds      []inbounds.Inbound
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	meta, draft, err := env.profiles.Create(context.Background(), profile.CreateInput{Name: "引用集成"})
	if err != nil {
		t.Fatal(err)
	}
	input := fixtures[0].Draft
	for key := range draft.Buffers {
		if _, ok := input.Buffers[key]; !ok {
			input.Buffers[key] = ""
		}
	}
	input.Buffers["inbounds"] = fixtures[7].Draft.Buffers["inbounds"]
	input.Buffers["route"] = fixtures[7].Draft.Buffers["route"]
	input.EditorState["inbounds"] = fixtures[7].Draft.EditorState["inbounds"]
	saved, _, err := env.profiles.SaveDraft(context.Background(), meta.ID, profile.RevisionETag(draft.Revision), profile.DraftInput{Buffers: input.Buffers, EditorState: input.EditorState})
	if err != nil {
		t.Fatal(err)
	}
	return meta.ID, saved.Revision, &referenceLibrary{subscription: fixtures[0].Subscriptions[0], inbound: fixtures[7].Inbounds[0], reads: map[string]int{}}
}

func TestReferencesDirectOperationsAndAutoReload(t *testing.T) {
	for _, running := range []bool{false, true} {
		name := "stopped"
		if running {
			name = "running"
		}
		t.Run(name, func(t *testing.T) {
			env := newIntegrationEnvironment(t)
			id, rev, library := saveReferenceFixture(t, env)
			factory := &countingProcessFactory{}
			manager, coordinator, _, _ := newApplicationCoordinatorWithFactory(t, env, nil, factory, configgen.WithReferenceSources(inboundLibrary{library}, subscriptionLibrary{library}))
			t.Cleanup(func() { _ = manager.Stop(context.Background()) })
			ctx := context.Background()
			if !env.config.ServerReferenceResolution() {
				t.Fatal("capability not enabled")
			}
			before, _, _ := env.profiles.GetDraft(ctx, id)
			beforeBytes, _ := json.Marshal(before)
			for _, operation := range []string{"preview", "validate"} {
				request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/"+id+"/"+operation, strings.NewReader(`{"revision":`+jsonNumber(rev)+`}`))
				response := httptest.NewRecorder()
				env.config.ServeHTTP(response, request)
				if response.Code != 200 {
					t.Fatalf("%s: %d %s", operation, response.Code, response.Body.String())
				}
				if !strings.Contains(response.Body.String(), "new.example.test") || strings.Contains(response.Body.String(), `\"detour\"`) {
					t.Fatal(response.Body.String())
				}
				if operation == "validate" {
					var result configgen.ValidateResult
					_ = json.Unmarshal(response.Body.Bytes(), &result)
					if !result.Valid {
						t.Fatal(response.Body.String())
					}
				}
			}
			if _, err := os.Stat(env.layout.ConfigDir); !os.IsNotExist(err) {
				t.Fatal("preview/check changed formal directory")
			}
			applied, err := coordinator.Apply(ctx, application.ApplyRequest{ProfileID: id, Revision: rev})
			if err != nil || !applied.Applied {
				t.Fatalf("%+v %v", applied, err)
			}
			if running {
				if err := coordinator.StartCore(ctx); err != nil {
					t.Fatal(err)
				}
			}
			pid := manager.Status(ctx).PID
			starts := factory.starts
			// Same saved revision; source deletes a field, changes a port and adds a node.
			library.subscription.Outbounds = []json.RawMessage{json.RawMessage(`{"type":"ssh","tag":"new","server":"changed.example.test","future":9007199254740997}`)}
			library.inbound.Config = json.RawMessage(`{"type":"mixed","tag":"renamed","listen_port":19099,"sniff":true}`)
			manual, err := coordinator.Apply(ctx, application.ApplyRequest{ProfileID: id, Revision: rev})
			if err != nil || !manual.Applied {
				t.Fatalf("manual=%+v %v", manual, err)
			}
			if manager.Status(ctx).PID != pid || factory.starts != starts {
				t.Fatal("manual apply restarted core")
			}
			payload, err := os.ReadFile(filepath.Join(env.layout.ConfigDir, "70-outbounds.json"))
			if err != nil || !bytes.Contains(payload, []byte("changed.example.test")) || bytes.Contains(payload, []byte("detour")) || !bytes.Contains(payload, []byte("9007199254740997")) {
				t.Fatalf("%s %v", payload, err)
			}
			if !bytes.Equal(payload, manual.Validation.Files[fileIndex(manual.Validation.Files, "70-outbounds.json")].Content) {
				t.Fatal("committed different JSON from checked candidate")
			}
			library.subscription.Outbounds = []json.RawMessage{json.RawMessage(`{"type":"ssh","tag":"new","server":"auto.example.test"}`)}
			automatic, err := coordinator.AutoReload(ctx, application.ApplyRequest{ProfileID: id, Revision: rev})
			if err != nil || !automatic.Applied || automatic.Reloaded != running {
				t.Fatalf("auto=%+v %v", automatic, err)
			}
			if factory.starts != starts+boolInt(running) {
				t.Fatal("incorrect restart count")
			}
			after, _, _ := env.profiles.GetDraft(ctx, id)
			afterBytes, _ := json.Marshal(after)
			if !bytes.Equal(beforeBytes, afterBytes) {
				t.Fatal("generation saved draft")
			}
			for key, count := range library.reads {
				if count != 5 {
					t.Fatalf("%s read %d times, expected once per operation", key, count)
				}
			}
		})
	}
}
func jsonNumber(v uint64) string { b, _ := json.Marshal(v); return string(b) }
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func fileIndex(files []configgen.File, name string) int {
	for i, f := range files {
		if f.Name == name {
			return i
		}
	}
	return -1
}

func TestCandidateFreezeUsesDisplayedRawTextAndETag(t *testing.T) {
	env := newIntegrationEnvironment(t)
	id, revision, library := saveReferenceFixture(t, env)
	env.config = configgen.NewHandler(env.layout, env.profiles, env.rules, nil, nil, nil, nil, configgen.WithReferenceSources(inboundLibrary{library}, subscriptionLibrary{library}))
	env.profiles.SetFreezeVerifier(env.config)
	ctx := context.Background()
	preview, err := env.config.Preview(ctx, id, revision)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Candidate == nil || preview.Candidate.ProfileID != id || preview.Candidate.Revision != revision {
		t.Fatalf("candidate=%+v", preview.Candidate)
	}
	var linked configgen.CandidateItem
	for _, item := range preview.Candidate.Items {
		if item.Module == "inbounds" && item.Tag == "local" {
			linked = item
			break
		}
	}
	if linked.Independent || linked.Token == "" || linked.ListenPort.String() != "19091" || !strings.Contains(linked.Text, "9007199254740997") {
		t.Fatalf("linked=%+v", linked)
	}
	request := profile.FreezeOperation{Action: "copy", Module: "inbounds", Tag: linked.Tag, NewTag: "local-copy", Text: linked.Text, RouteText: linked.RouteText, InboundStateText: linked.InboundStateText, Digest: preview.Digest, ResourceDigests: preview.ResourceDigests, Token: linked.Token}
	// Source changes after display: the save still fixes exactly the displayed
	// candidate and never rereads the resource.
	library.inbound.Config = json.RawMessage(`{"type":"mixed","tag":"source","listen_port":19099,"future":9007199254740999}`)
	readsBefore := library.reads["inbound:"+library.inbound.ID]
	payload, _ := json.Marshal(profile.DraftInput{Freeze: &request})
	httpRequest := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+id+"/draft", bytes.NewReader(payload))
	httpRequest.Header.Set("If-Match", profile.RevisionETag(revision))
	httpResponse := httptest.NewRecorder()
	profile.NewHandler(env.profiles).ServeHTTP(httpResponse, httpRequest)
	if httpResponse.Code != http.StatusOK {
		t.Fatal(httpResponse.Code, httpResponse.Body.String())
	}
	var saved profile.Draft
	if err := json.Unmarshal(httpResponse.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if httpResponse.Header().Get("ETag") != profile.RevisionETag(saved.Revision) {
		t.Fatal("missing ETag")
	}
	if library.reads["inbound:"+library.inbound.ID] != readsBefore {
		t.Fatal("freeze reread source")
	}
	if !strings.Contains(saved.Buffers["inbounds"], `"future":9007199254740997`) || strings.Contains(saved.Buffers["inbounds"], `9007199254740999`) || !strings.Contains(saved.Buffers["inbounds"], `"tag":"local-copy"`) {
		t.Fatal(saved.Buffers["inbounds"])
	}
	if !strings.Contains(saved.Buffers["route"], `"inbound":"local-copy"`) {
		t.Fatal(saved.Buffers["route"])
	}
	if _, _, err := env.profiles.SaveDraft(ctx, id, profile.RevisionETag(revision), profile.DraftInput{Freeze: &request}); err == nil {
		t.Fatal("stale ETag accepted")
	}
	// An altered raw token is rejected even if the candidate digest is copied.
	current, _, err := env.profiles.GetDraft(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	bad := request
	bad.Text = strings.Replace(request.Text, "19091", "19092", 1)
	if _, _, err := env.profiles.SaveDraft(ctx, id, profile.RevisionETag(current.Revision), profile.DraftInput{Freeze: &bad}); err == nil {
		t.Fatal("altered candidate accepted")
	}
}

func TestExplicitSavedCopyRecoveryAfterSourceDeletion(t *testing.T) {
	env := newIntegrationEnvironment(t)
	id, revision, library := saveReferenceFixture(t, env)
	env.config = configgen.NewHandler(env.layout, env.profiles, env.rules, nil, nil, nil, nil, configgen.WithReferenceSources(inboundLibrary{library}, subscriptionLibrary{library}))
	env.profiles.SetFreezeVerifier(env.config)
	draft, _, err := env.profiles.GetDraft(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	previous := draft.Buffers["inbounds"]
	operation := profile.FreezeOperation{Action: "restore_saved", Module: "inbounds", Tag: "local"}
	saved, _, err := env.profiles.SaveDraft(context.Background(), id, profile.RevisionETag(revision), profile.DraftInput{Freeze: &operation})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Buffers["inbounds"] != previous || strings.Contains(string(saved.EditorState["inbounds"]), "resourceBindings") {
		t.Fatal("saved copy recovery changed content or kept binding")
	}
}

func TestVersionTwoReferencesGenerateWithoutSavedExpansion(t *testing.T) {
	env := newIntegrationEnvironment(t)
	id, revision, library := saveReferenceFixture(t, env)
	ctx := context.Background()
	old, etag, err := env.profiles.GetDraft(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	migrated, _, err := env.profiles.SaveDraft(ctx, id, etag, profile.DraftInput{SchemaVersion: 2, Buffers: old.Buffers, EditorState: old.EditorState, Bindings: old.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Revision != revision+1 || migrated.References == nil || migrated.SchemaVersion != 2 {
		t.Fatal(migrated.References)
	}
	if strings.Contains(migrated.Buffers["outbounds"], `"tag":"old"`) || strings.Contains(migrated.Buffers["inbounds"], `"tag":"local"`) {
		t.Fatal("generated copies persisted", migrated.Buffers)
	}
	env.config = configgen.NewHandler(env.layout, env.profiles, env.rules, nil, nil, nil, nil, configgen.WithReferenceSources(inboundLibrary{library}, subscriptionLibrary{library}))
	first, err := env.config.Preview(ctx, id, migrated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if first.Candidate == nil || !strings.Contains(string(first.Files[fileIndex(first.Files, "70-outbounds.json")].Content), "new.example.test") || !strings.Contains(string(first.Files[fileIndex(first.Files, "60-inbounds.json")].Content), "19091") {
		t.Fatal(first)
	}
	library.subscription.Outbounds = []json.RawMessage{json.RawMessage(`{"type":"ssh","tag":"new","server":"changed.example.test"}`)}
	library.inbound.Config = json.RawMessage(`{"type":"mixed","tag":"renamed","listen_port":19099,"sniff":true}`)
	second, err := env.config.Preview(ctx, id, migrated.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest == first.Digest || !strings.Contains(string(second.Files[fileIndex(second.Files, "70-outbounds.json")].Content), "changed.example.test") || !strings.Contains(string(second.Files[fileIndex(second.Files, "60-inbounds.json")].Content), "19099") {
		t.Fatal(second)
	}
	third, err := env.config.Preview(ctx, id, migrated.Revision)
	if err != nil || third.Digest != second.Digest {
		t.Fatalf("nondeterministic v2 result: %v", err)
	}
	current, _, err := env.profiles.GetDraft(ctx, id)
	if err != nil || current.Revision != migrated.Revision || current.Buffers["outbounds"] != migrated.Buffers["outbounds"] {
		t.Fatal("preview changed persisted input")
	}
}

func TestVersionTwoFreezeAndExplicitBackupRecovery(t *testing.T) {
	for _, action := range []string{"detach", "copy", "restore_saved"} {
		t.Run(action, func(t *testing.T) {
			env := newIntegrationEnvironment(t)
			id, _, library := saveReferenceFixture(t, env)
			ctx := context.Background()
			legacy, etag, err := env.profiles.GetDraft(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			migrated, _, err := env.profiles.SaveDraft(ctx, id, etag, profile.DraftInput{SchemaVersion: 2, Buffers: legacy.Buffers, EditorState: legacy.EditorState, Bindings: legacy.Bindings})
			if err != nil {
				t.Fatal(err)
			}
			env.config = configgen.NewHandler(env.layout, env.profiles, env.rules, nil, nil, nil, nil, configgen.WithReferenceSources(inboundLibrary{library}, subscriptionLibrary{library}))
			env.profiles.SetFreezeVerifier(env.config)
			operation := profile.FreezeOperation{Action: action, Module: "inbounds", Tag: "local"}
			if action != "restore_saved" {
				preview, err := env.config.Preview(ctx, id, migrated.Revision)
				if err != nil {
					t.Fatal(err)
				}
				var item configgen.CandidateItem
				for _, candidate := range preview.Candidate.Items {
					if candidate.Module == "inbounds" && candidate.Tag == "local" {
						item = candidate
						break
					}
				}
				operation.Text = item.Text
				operation.RouteText = item.RouteText
				operation.InboundStateText = item.InboundStateText
				operation.Digest = preview.Digest
				operation.ResourceDigests = preview.ResourceDigests
				operation.Token = item.Token
				if action == "copy" {
					operation.NewTag = "local-copy"
				}
			}
			saved, _, err := env.profiles.SaveDraft(ctx, id, profile.RevisionETag(migrated.Revision), profile.DraftInput{SchemaVersion: 2, Freeze: &operation})
			if err != nil {
				t.Fatal(err)
			}
			if saved.SchemaVersion != 2 || saved.References == nil {
				t.Fatal(saved)
			}
			if action == "copy" {
				if len(saved.References.Inbounds) != 1 || !strings.Contains(saved.Buffers["inbounds"], `"tag":"local-copy"`) {
					t.Fatal(saved)
				}
			} else if len(saved.References.Inbounds) != 0 {
				t.Fatal("binding remained")
			}
			if !strings.Contains(saved.Buffers["inbounds"], "900719925474099") || (action != "restore_saved" && !strings.Contains(saved.Buffers["route"], `"action":"sniff"`)) {
				t.Fatal("candidate/backup lost raw fields or sniff")
			}
		})
	}
}
