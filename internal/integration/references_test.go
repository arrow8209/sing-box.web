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
