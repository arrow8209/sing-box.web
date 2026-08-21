package profile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRepairsMetaRevisionFromHealthyDraft(t *testing.T) {
	service, layout := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "恢复"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(layout.ProfilesFile, []byte(`{"version":1,"profiles":[{"id":"`+meta.ID+`","name":"恢复","draftRevision":1,"appliedDigest":"","appliedResourceDigests":{},"needsApply":true}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(layout, service.templates, service.defaultBindings, testClock, service.random)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := reopened.List(context.Background())
	if err != nil || len(profiles) != 1 || profiles[0].DraftRevision != 2 {
		t.Fatalf("profiles=%+v err=%v", profiles, err)
	}
	if _, err := os.Stat(filepath.Join(layout.DraftsDir, meta.ID+".json")); err != nil {
		t.Fatal(err)
	}
}
