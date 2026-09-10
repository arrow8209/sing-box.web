package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/application"
	"singbox-web/internal/configgen"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	"singbox-web/internal/templates"
)

type integrationEnvironment struct {
	workDir  string
	layout   paths.Layout
	profiles *profile.Service
	rules    *rules.Service
	config   *configgen.Handler
	argsFile string
	control  string
}

type noBindings struct{}

func (noBindings) DefaultBindings(context.Context) ([]profile.RuleBinding, error) { return nil, nil }

func newIntegrationEnvironment(t *testing.T) *integrationEnvironment {
	t.Helper()
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := templates.NewFactory(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := profile.Open(layout, factory, noBindings{}, time.Now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store, err := rules.OpenStore(layout.RulesFile, layout, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := rules.NewService(store, layout, profiles, time.Now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &integrationEnvironment{workDir: workDir, layout: layout, profiles: profiles, rules: resources, argsFile: filepath.Join(workDir, "fake-core-args.jsonl"), control: filepath.Join(workDir, "fake-core-control.json")}
}

func (e *integrationEnvironment) InstallFakeCore(t *testing.T, behavior fakeCoreBehavior) {
	t.Helper()
	if err := os.MkdirAll(e.layout.BinDir, 0o700); err != nil {
		t.Fatal(err)
	}
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(current)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(e.layout.CoreBinary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	behavior.ArgsFile = e.argsFile
	contents, err := json.Marshal(behavior)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.control, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	checker := configgen.NewChecker(e.workDir, configgen.ExecRunner{Environment: []string{"SINGBOX_WEB_TEST_HELPER=1", "SINGBOX_WEB_TEST_CONTROL=" + e.control}}, 5*time.Second, 64<<10)
	e.config = configgen.NewHandler(e.layout, e.profiles, e.rules, nil, checker, nil, time.Now)
}

func (e *integrationEnvironment) CreateRuleResource(t *testing.T, url string) string {
	t.Helper()
	resource, err := e.rules.Create(context.Background(), rules.CreateInput{Name: "binary", URL: url, Format: rules.FormatBinary, UpdateInterval: "1h"})
	if err != nil {
		t.Fatal(err)
	}
	return resource.ID
}

func (e *integrationEnvironment) CreateBoundProfile(t *testing.T, resourceID, tag string) (string, uint64) {
	t.Helper()
	meta, draft, err := e.profiles.Create(context.Background(), profile.CreateInput{Name: "集成测试"})
	if err != nil {
		t.Fatal(err)
	}
	buffers := make(map[string]string, len(draft.Buffers))
	for module := range draft.Buffers {
		buffers[module] = ""
	}
	buffers["route"] = `{}`
	buffers["inbounds"] = `[]`
	buffers["outbounds"] = `[]`
	updated, _, err := e.profiles.SaveDraft(context.Background(), meta.ID, profile.RevisionETag(draft.Revision), profile.DraftInput{Buffers: buffers, Bindings: []profile.RuleBinding{{ResourceID: resourceID, Tag: tag, Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	return meta.ID, updated.Revision
}

func (e *integrationEnvironment) FakeCoreArguments(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(e.argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(contents))
	if len(lines) != 1 {
		t.Fatalf("fake core calls=%q", contents)
	}
	var args []string
	if err := json.Unmarshal([]byte(lines[0]), &args); err != nil {
		t.Fatal(err)
	}
	return args
}

func TestRuleDownloadPreviewAndValidateFlow(t *testing.T) {
	env := newIntegrationEnvironment(t)
	env.InstallFakeCore(t, fakeCoreBehavior{Version: "sing-box version 1.12.0", CheckExit: 0})
	ruleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("binary-rule-data"))
	}))
	defer ruleServer.Close()
	resourceID := env.CreateRuleResource(t, ruleServer.URL)
	update, err := env.rules.UpdateNow(context.Background(), resourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Changed {
		t.Fatalf("首次下载未标记为变更")
	}
	profileID, revision := env.CreateBoundProfile(t, resourceID, "GeoSite-Test")
	preview, err := env.config.Preview(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	assertPreviewLocalRule(t, preview, "GeoSite-Test", "data/rulesets/"+resourceID+".srs")
	validated, err := env.config.Validate(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	if !validated.Valid {
		t.Fatalf("validate=%+v", validated)
	}
	wantStaging := "config.staging-" + profileID
	if got := env.FakeCoreArguments(t); len(got) != 3 || !slices.Equal(got[:2], []string{"check", "-C"}) || got[2] != wantStaging || filepath.IsAbs(got[2]) {
		t.Fatalf("fake core args=%v", got)
	}
	assertPersistentStaging(t, env.workDir, profileID)
	assertNoTransactionDirs(t, env.workDir)
	metas, err := env.profiles.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 1 || metas[0].LastValidation == nil || !metas[0].LastValidation.Valid || metas[0].LastValidation.Revision != revision {
		t.Fatalf("validation was not recorded: %+v", metas)
	}
	if _, err := os.Stat(env.layout.ConfigDir); !os.IsNotExist(err) {
		t.Fatalf("正式 config 被预览或检查创建: %v", err)
	}
}

func TestManualRulePreviewAndValidateFlow(t *testing.T) {
	env := newIntegrationEnvironment(t)
	env.InstallFakeCore(t, fakeCoreBehavior{Version: "sing-box version 1.12.0", CheckExit: 0})
	resource, err := env.rules.Create(context.Background(), rules.CreateInput{
		SourceType: rules.SourceManual,
		Name:       "手工私有域名",
		Content:    json.RawMessage(`{"version":3,"rules":[{"domain_suffix":["example.com","example.org"]}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	profileID, revision := env.CreateBoundProfile(t, resource.ID, "Manual-Domains")
	preview, err := env.config.Preview(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	assertPreviewLocalRule(t, preview, "Manual-Domains", "data/rulesets/"+resource.ID+".json")
	validated, err := env.config.Validate(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	if !validated.Valid {
		t.Fatalf("validate=%+v", validated)
	}
	if validated.ConfigDirectory != "config.staging-"+profileID {
		t.Fatalf("config directory=%q", validated.ConfigDirectory)
	}
	assertPersistentStaging(t, env.workDir, profileID)
	payload, err := os.ReadFile(filepath.Join(env.layout.RuleSetsDir, resource.ID+".json"))
	if err != nil || !bytes.Contains(payload, []byte("example.org")) {
		t.Fatalf("manual payload=%s err=%v", payload, err)
	}
	assertNoTransactionDirs(t, env.workDir)
}

func TestPreviewValidationAndApplyExcludeImportedOutboundGUIIdentifiers(t *testing.T) {
	env := newIntegrationEnvironment(t)
	env.InstallFakeCore(t, fakeCoreBehavior{Version: "sing-box version 1.13.19", CheckExit: 0})
	profileID, _ := createApplicationProfile(t, env)
	draft, etag, err := env.profiles.GetDraft(context.Background(), profileID)
	if err != nil {
		t.Fatal(err)
	}
	const imported = `[{"type":"direct","tag":"direct","__id_in_gui":"imported-node-id"}]`
	draft.Buffers["outbounds"] = imported
	saved, _, err := env.profiles.SaveDraft(context.Background(), profileID, etag, profile.DraftInput{Buffers: draft.Buffers})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := env.config.Preview(context.Background(), profileID, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := env.config.Validate(context.Background(), profileID, saved.Revision)
	if err != nil || !validated.Valid {
		t.Fatalf("validate=%+v err=%v", validated, err)
	}
	if preview.Digest != validated.Digest {
		t.Fatal("preview and validation generated different configurations")
	}
	stagingFile := filepath.Join(env.workDir, validated.ConfigDirectory, "70-outbounds.json")
	checked, err := os.ReadFile(stagingFile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(checked, []byte("__id_in_gui")) {
		t.Fatal("GUI metadata was included in the configuration sent to the core")
	}
	var previewOutbounds []byte
	for _, file := range preview.Files {
		if file.Name == "70-outbounds.json" {
			previewOutbounds = file.Content
		}
	}
	if len(previewOutbounds) == 0 || !bytes.Equal(checked, previewOutbounds) {
		t.Fatal("the checked outbound configuration did not match its preview")
	}
	_, coordinator, _, _ := newApplicationCoordinator(t, env, nil)
	applied, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: saved.Revision})
	if err != nil || !applied.Applied || applied.Validation.Digest != preview.Digest {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	formal, err := os.ReadFile(filepath.Join(env.layout.ConfigDir, "70-outbounds.json"))
	if err != nil || !bytes.Equal(formal, previewOutbounds) {
		t.Fatalf("applied outbound configuration did not match its preview: err=%v", err)
	}
	stored, _, err := env.profiles.GetDraft(context.Background(), profileID)
	if err != nil || stored.Buffers["outbounds"] != imported || stored.Revision != saved.Revision {
		t.Fatalf("preview or validation modified the imported draft: err=%v", err)
	}
}

func assertPreviewLocalRule(t *testing.T, preview configgen.PreviewResult, tag, path string) {
	t.Helper()
	for _, file := range preview.Files {
		if file.Name == "80-route.json" && strings.Contains(string(file.Content), tag) && strings.Contains(string(file.Content), path) {
			return
		}
	}
	t.Fatalf("预览没有固定本地规则 %s: %+v", path, preview.Files)
}

func assertNoTransactionDirs(t *testing.T, workDir string) {
	t.Helper()
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config.rollback-") || strings.HasSuffix(entry.Name(), ".preparing") || strings.HasSuffix(entry.Name(), ".previous") {
			t.Fatalf("残留事务目录: %s", entry.Name())
		}
	}
}

func assertPersistentStaging(t *testing.T, workDir, profileID string) {
	t.Helper()
	directory := filepath.Join(workDir, "config.staging-"+profileID)
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("persistent staging=%s info=%v err=%v", directory, info, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "80-route.json")); err != nil {
		t.Fatalf("persistent staging missing generated config: %v", err)
	}
}
