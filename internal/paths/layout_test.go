package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/paths"
)

func TestResolveUsesOnlyFixedRelativePaths(t *testing.T) {
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, layout.DataDir, filepath.Join(root, "data"))
	assertPath(t, layout.ConfigDir, filepath.Join(root, "config"))
	assertPath(t, layout.CoreBinary, filepath.Join(root, "data", "bin", "sing-box"))
	assertPath(t, layout.SettingsFile, filepath.Join(root, "data", "settings.json"))
	assertPath(t, layout.SubscriptionsFile, filepath.Join(root, "data", "subscriptions.json"))
	assertPath(t, layout.InboundResourcesFile, filepath.Join(root, "data", "inbound-resources.json"))
	assertPath(t, layout.AppLock, filepath.Join(root, "data", "app.lock"))
}

func TestResolveRejectsManagedSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "data")); err != nil {
		t.Fatal(err)
	}
	if _, err := paths.Resolve(root); err == nil {
		t.Fatal("managed data symlink was accepted")
	}
}

func TestRuleSetPathUsesIDAndFormatOnly(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	absolute, relative, err := layout.RuleSetPath("0123456789abcdef0123456789abcdef", ".srs")
	if err != nil {
		t.Fatal(err)
	}
	if relative != "data/rulesets/0123456789abcdef0123456789abcdef.srs" {
		t.Fatalf("relative=%q", relative)
	}
	if absolute != filepath.Join(layout.WorkDir, filepath.FromSlash(relative)) {
		t.Fatalf("absolute=%q", absolute)
	}
	for _, bad := range []string{"../escape", "/absolute", "id/name"} {
		if _, _, err := layout.RuleSetPath(bad, ".srs"); err == nil {
			t.Fatalf("accepted id %q", bad)
		}
	}
	if _, _, err := layout.RuleSetPath("0123456789abcdef0123456789abcdef", ".exe"); err == nil {
		t.Fatal("accepted non-ruleset extension")
	}
}

func TestStagingConfigDirUsesProfileID(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profileID := "0123456789abcdef0123456789abcdef"
	got, err := layout.StagingConfigDir(profileID)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(layout.WorkDir, "config.staging-"+profileID) {
		t.Fatalf("staging=%q", got)
	}
	for _, invalid := range []string{"short", "0123456789abcdef0123456789abcdeg", "../0123456789abcdef0123456789abcdef"} {
		if _, err := layout.StagingConfigDir(invalid); err == nil {
			t.Fatalf("accepted profile ID %q", invalid)
		}
	}
}

func assertPath(t *testing.T, actual, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("path = %q, want %q", actual, expected)
	}
}
