package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/profile"
	"singbox-web/internal/publication"
	"singbox-web/internal/singbox"
	"singbox-web/internal/validation"
)

type configProfileStub struct {
	ProfileService
	item    profile.Profile
	draft   profile.Draft
	summary profile.ValidationSummary
}

func (stub *configProfileStub) Get(context.Context, string) (profile.Profile, error) {
	return stub.item, nil
}
func (stub *configProfileStub) GetDraft(context.Context, string) (profile.Draft, error) {
	return stub.draft, nil
}
func (stub *configProfileStub) RecordValidation(_ context.Context, _ string, revision uint64, summary profile.ValidationSummary) (profile.Profile, error) {
	stub.summary = summary
	if revision != stub.draft.Revision {
		return profile.Profile{}, &profile.RevisionConflict{Current: stub.draft.Revision}
	}
	return stub.item, nil
}

type preflightStub struct {
	result validation.Result
	calls  int
}

func (stub *preflightStub) ValidateDraft(profile.Draft) validation.Result {
	stub.calls++
	return stub.result
}

type publicationStub struct {
	PublicationService
	receipt      publication.Receipt
	err          error
	publishCalls int
	deletedName  string
}

type httpUnsafeChecker struct{ root string }

func (checker httpUnsafeChecker) Version(context.Context) (string, error) { return "v", nil }
func (checker httpUnsafeChecker) Check(_ context.Context, staging string) (singbox.CheckResult, error) {
	output := "secret=do-not-leak " + checker.root + " " + staging + " " + strings.Repeat("x", 70*1024)
	return singbox.CheckResult{Stderr: output}, fmt.Errorf("raw-do-not-leak: %w", singbox.ErrCheckFailed)
}

func (stub *publicationStub) Publish(_ context.Context, _ publication.Request) (publication.Receipt, error) {
	stub.publishCalls++
	return stub.receipt, stub.err
}
func (stub *publicationStub) ReadPublished(context.Context, string) (json.RawMessage, error) {
	return nil, publication.ErrNotPublished
}
func (stub *publicationStub) Delete(_ context.Context, _ string, name string) error {
	stub.deletedName = name
	return stub.err
}

func configRequest(method, target, body, id string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.SetPathValue("id", id)
	return request
}

func TestValidateRecordsExactSafeRevision(t *testing.T) {
	id := strings.Repeat("a", 32)
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	profiles := &configProfileStub{item: profile.Profile{ID: id}, draft: profile.Draft{ProfileID: id, Revision: 7}}
	preflight := &preflightStub{result: validation.Result{Issues: []validation.Issue{{Stage: "schema", Message: "secret"}}, CheckedAt: now}}
	service := server{deps: Dependencies{Profiles: profiles, Preflight: preflight, Now: func() time.Time { return now }}}
	response := httptest.NewRecorder()
	service.validateProfile(response, configRequest(http.MethodPost, "/api/v1/profiles/"+id+"/validate", "", id))
	if response.Code != http.StatusOK || preflight.calls != 1 || profiles.summary.Revision != 7 || profiles.summary.Message != "校验失败，共 1 项" || strings.Contains(profiles.summary.Message, "secret") {
		t.Fatalf("status=%d summary=%#v", response.Code, profiles.summary)
	}
}

func TestPublishErrorClassificationIsSafe(t *testing.T) {
	id := strings.Repeat("b", 32)
	now := time.Now()
	profiles := &configProfileStub{item: profile.Profile{ID: id}}
	publicationService := &publicationStub{err: errors.New("raw secret /absolute/root")}
	service := server{deps: Dependencies{Profiles: profiles, Publication: publicationService, Now: func() time.Time { return now }}}
	request := configRequest(http.MethodPost, "/api/v1/profiles/"+id+"/publish", `{}`, id)
	request.Header.Set("If-Match", `"draft-1"`)
	response := httptest.NewRecorder()
	service.publishProfile(response, request)
	if response.Code != http.StatusInternalServerError || publicationService.publishCalls != 1 || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "/absolute") {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestPublishReturnsStableExternalDriftConflict(t *testing.T) {
	id := strings.Repeat("d", 32)
	now := time.Now()
	profiles := &configProfileStub{item: profile.Profile{ID: id}}
	publicationService := &publicationStub{err: publication.ErrExternalDrift}
	service := server{deps: Dependencies{Profiles: profiles, Publication: publicationService, Now: func() time.Time { return now }}}
	request := configRequest(http.MethodPost, "/api/v1/profiles/"+id+"/publish", `{}`, id)
	request.Header.Set("If-Match", `"draft-1"`)
	response := httptest.NewRecorder()

	service.publishProfile(response, request)

	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusConflict || body.Error.Code != "external_drift" || body.Error.Message != "正式配置已被外部修改，需要确认后覆盖" || publicationService.publishCalls != 1 {
		t.Fatalf("status=%d body=%#v calls=%d", response.Code, body, publicationService.publishCalls)
	}
}

func TestPublishReturnsOnlyStructuredSanitizedCheckerOutput(t *testing.T) {
	harness := newProfileHarness(t)
	item, _, err := harness.service.Create(context.Background(), profile.CreateInput{Name: "安全输出", Slug: "safe-output"})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := validation.NewSchemaValidator(assets.SingBoxSchema)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now
	publicationService := publication.NewService(publication.Options{
		ConfigRoot: harness.configRoot,
		Profiles:   harness.repo,
		Preflight:  validation.NewService(schema, "target", now),
		Checker:    httpUnsafeChecker{root: harness.configRoot},
		Now:        now,
		Random:     bytes.NewReader(bytes.Repeat([]byte{3}, 256)),
	})
	service := server{deps: Dependencies{Profiles: harness.service, Publication: publicationService, Now: now}}
	request := configRequest(http.MethodPost, "/api/v1/profiles/"+item.ID+"/publish", `{}`, item.ID)
	request.Header.Set("If-Match", `"draft-1"`)
	response := httptest.NewRecorder()
	service.publishProfile(response, request)
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusUnprocessableEntity || body.Error.Code != "singbox_check_failed" || strings.Contains(body.Error.Stderr, "do-not-leak") || strings.Contains(body.Error.Stderr, harness.configRoot) || !strings.Contains(body.Error.Stderr, "REDACTED") || len([]byte(body.Error.Stderr)) > 64*1024+64 {
		t.Fatalf("status=%d code=%q stderrBytes=%d prefix=%q", response.Code, body.Error.Code, len([]byte(body.Error.Stderr)), body.Error.Stderr[:min(len(body.Error.Stderr), 160)])
	}
}

func TestExportAndDeleteValidateInputs(t *testing.T) {
	id := strings.Repeat("c", 32)
	now := time.Now()
	profiles := &configProfileStub{item: profile.Profile{ID: id, Name: "当前显示名称", Slug: "profile"}, draft: profile.Draft{Revision: 1, Content: profile.DraftContent{Config: json.RawMessage(`{"password":"secret"}`)}}}
	publicationService := &publicationStub{}
	service := server{deps: Dependencies{Profiles: profiles, Publication: publicationService, Now: func() time.Time { return now }}}
	badExport := httptest.NewRecorder()
	service.exportProfile(badExport, configRequest(http.MethodGet, "/api/v1/profiles/"+id+"/export?mode=unknown", "", id))
	if badExport.Code != http.StatusBadRequest {
		t.Fatalf("export status=%d", badExport.Code)
	}
	badDelete := httptest.NewRecorder()
	service.deleteProfile(badDelete, configRequest(http.MethodDelete, "/api/v1/profiles/"+id, `{"confirmName":"错误"}`, id))
	if badDelete.Code != http.StatusConflict || publicationService.deletedName != "" {
		t.Fatalf("delete status=%d", badDelete.Code)
	}
}
