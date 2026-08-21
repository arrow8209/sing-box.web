//go:build contract

package contracttest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/configgen"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	"singbox-web/internal/templates"
)

type selection struct {
	Version       string `json:"version"`
	ReleaseURL    string `json:"releaseURL"`
	AssetName     string `json:"assetName"`
	AssetURL      string `json:"assetURL"`
	ArchiveSHA256 string `json:"archiveSHA256"`
	BinarySHA256  string `json:"binarySHA256"`
}

func TestEmbeddedTemplatePassesManagedSingBoxCheck(t *testing.T) {
	binary := os.Getenv("SINGBOX_CONTRACT_BINARY")
	if binary == "" {
		t.Fatal("SINGBOX_CONTRACT_BINARY is required")
	}
	selected := readSelection(t)
	versionOutput := runBounded(t, binary, "version")
	if version := parseVersion(versionOutput); version != selected.Version {
		t.Fatalf("contract binary version=%q, want %q; output=%s", version, selected.Version, versionOutput)
	}
	if digest := sha256File(t, binary); digest != selected.BinarySHA256 {
		t.Fatalf("contract binary SHA-256=%s, want %s", digest, selected.BinarySHA256)
	}

	workDir := t.TempDir()
	draft, snapshot, release := createContractDraftAndLocalRules(t, workDir)
	defer release()
	output, err := configgen.NewGenerator().Generate(context.Background(), draft, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(workDir, "config")
	if err := configgen.NewGenerator().WriteDirectory(context.Background(), output, configDir); err != nil {
		t.Fatal(err)
	}
	checkOutput := runBoundedInDir(t, workDir, binary, "check", "-C", configDir)
	t.Logf("sing-box=%s binary_sha256=%s output=%s", selected.Version, selected.BinarySHA256, checkOutput)
}

func readSelection(t *testing.T) selection {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected selection
	if err := json.Unmarshal(payload, &selected); err != nil {
		t.Fatal(err)
	}
	if selected.Version == "" || selected.ReleaseURL == "" || selected.AssetName == "" || selected.AssetURL == "" || len(selected.ArchiveSHA256) != 64 || len(selected.BinarySHA256) != 64 {
		t.Fatalf("invalid contract selection: %+v", selected)
	}
	return selected
}

func createContractDraftAndLocalRules(t *testing.T, workDir string) (profile.Draft, rules.Snapshot, func()) {
	t.Helper()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := templates.NewFactory(nil)
	if err != nil {
		t.Fatal(err)
	}
	store, err := rules.OpenStore(layout.RulesFile, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	service, err := rules.NewService(store, layout, contractProfiles{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := store.EnsureTemplateResources(context.Background(), factory.ResourceTemplates(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, template := range factory.ResourceTemplates() {
		if _, err := service.UpdateNow(context.Background(), ids[template.TemplateKey]); err != nil {
			t.Fatalf("下载默认规则 %s: %v", template.Name, err)
		}
	}
	seed, err := factory.NewProfileSeed(time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	bindings := make([]profile.RuleBinding, 0, len(factory.BindingTemplates()))
	for _, binding := range factory.BindingTemplates() {
		id := ids[binding.TemplateKey]
		if id == "" {
			t.Fatalf("默认规则绑定 %s 没有资源", binding.Tag)
		}
		bindings = append(bindings, profile.RuleBinding{ResourceID: id, Tag: binding.Tag, Enabled: binding.Enabled})
	}
	draft := profile.Draft{Config: seed.Config, Buffers: seed.Buffers, Bindings: bindings}
	snapshot, release, err := service.SnapshotForBindings(context.Background(), bindings)
	if err != nil {
		t.Fatal(err)
	}
	return draft, snapshot, release
}

func runBounded(t *testing.T, binary string, args ...string) string {
	return runBoundedInDir(t, "", binary, args...)
}

func runBoundedInDir(t *testing.T, directory, binary string, args ...string) string {
	t.Helper()
	context, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(context, binary, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if context.Err() != nil {
		t.Fatalf("sing-box %s timed out", strings.Join(args, " "))
	}
	if err != nil {
		t.Fatalf("sing-box %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

type contractProfiles struct{}

func (contractProfiles) ReferencesForResource(context.Context, string) ([]rules.Reference, error) {
	return nil, nil
}

func parseVersion(output string) string {
	fields := strings.Fields(output)
	for index := 0; index+2 < len(fields); index++ {
		if fields[index] == "sing-box" && fields[index+1] == "version" {
			return fields[index+2]
		}
	}
	return ""
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
