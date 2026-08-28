package profile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/paths"
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

func TestValidateManagedStateRejectsDuplicateIDsAndInvalidRevision(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "duplicate", body: `{"version":1,"profiles":[{"id":"` + strings.Repeat("a", 32) + `","name":"A","draftRevision":1,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null},{"id":"` + strings.Repeat("a", 32) + `","name":"B","draftRevision":1,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null}]}`},
		{name: "revision", body: `{"version":1,"profiles":[{"id":"` + strings.Repeat("b", 32) + `","name":"A","draftRevision":0,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null}]}`},
		{name: "multiple current configs", body: `{"version":1,"profiles":[{"id":"` + strings.Repeat("c", 32) + `","name":"A","draftRevision":1,"appliedRevision":1,"appliedDigest":"a","appliedResourceDigests":{},"appliedAt":null,"configCurrent":true,"needsApply":false,"lastValidation":null},{"id":"` + strings.Repeat("d", 32) + `","name":"B","draftRevision":1,"appliedRevision":1,"appliedDigest":"b","appliedResourceDigests":{},"appliedAt":null,"configCurrent":true,"needsApply":false,"lastValidation":null}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout, err := paths.Resolve(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(layout.ProfilesFile), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(layout.ProfilesFile, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateManagedState(layout); err == nil || !strings.Contains(err.Error(), layout.ProfilesFile) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
