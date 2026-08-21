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
	e.config = configgen.NewHandler(e.layout, e.profiles, e.rules, nil, checker, nil, time.Now, rand.Reader)
}

func (e *integrationEnvironment) CreateRuleResource(t *testing.T, url string) string {
	t.Helper()
	resource, err := e.rules.Create(context.Background(), rules.CreateInput{Name: "source", URL: url, Format: rules.FormatSource, UpdateInterval: "1h"})
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
		_, _ = w.Write([]byte(`{"version":3,"rules":[]}`))
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
	assertPreviewLocalRule(t, preview, "GeoSite-Test", "data/rulesets/"+resourceID+".json")
	validated, err := env.config.Validate(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	if !validated.Valid {
		t.Fatalf("validate=%+v", validated)
	}
	if got := env.FakeCoreArguments(t); len(got) != 3 || !slices.Equal(got[:2], []string{"check", "-C"}) || !strings.HasPrefix(got[2], filepath.Join(env.workDir, "config.staging-")) {
		t.Fatalf("fake core args=%v", got)
	}
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
	payload, err := os.ReadFile(filepath.Join(env.layout.RuleSetsDir, resource.ID+".json"))
	if err != nil || !bytes.Contains(payload, []byte("example.org")) {
		t.Fatalf("manual payload=%s err=%v", payload, err)
	}
	assertNoTransactionDirs(t, env.workDir)
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
		if strings.HasPrefix(entry.Name(), "config.staging-") || strings.HasPrefix(entry.Name(), "config.rollback-") {
			t.Fatalf("残留事务目录: %s", entry.Name())
		}
	}
}
