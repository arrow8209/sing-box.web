package rules

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverRuleTransactionsRestoresMatchingRollback(t *testing.T) {
	service, layout := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	fixed, _, err := layout.RuleSetPath(resource.ID, ".srs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.RuleSetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := []byte("old")
	sum := sha256.Sum256(old)
	if err := os.WriteFile(fixed, []byte("uncommitted"), 0o600); err != nil {
		t.Fatal(err)
	}
	rollback := filepath.Join(layout.RuleSetsDir, "."+filepath.Base(fixed)+".rollback-0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(rollback, old, 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := service.store.list()
	if err != nil {
		t.Fatal(err)
	}
	resources[0].LastDigest = fmt.Sprintf("%x", sum)
	if err := service.store.write(resources); err != nil {
		t.Fatal(err)
	}
	if err := RecoverRuleTransactions(service.store, layout); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(fixed)
	if err != nil || string(got) != string(old) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}
