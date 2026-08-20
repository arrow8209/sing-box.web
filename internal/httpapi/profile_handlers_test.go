package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/auth"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/profile"
)

type profileHarness struct {
	handler    http.Handler
	service    *profile.Service
	repo       *profile.Repository
	configRoot string
	cookie     *http.Cookie
	csrf       string
}

func newProfileHarness(t *testing.T) profileHarness {
	t.Helper()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := rand.Reader
	dataDir := t.TempDir()
	repo, err := profile.OpenRepository(dataDir, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	profiles := profile.NewService(repo, template, random)
	store := auth.NewFileStore(filepath.Join(dataDir, "auth.json"), func() time.Time { return now }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := auth.NewSessionManager(store, func() time.Time { return now }, random)
	configRoot := filepath.Join(t.TempDir(), "config root", "it's")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	handler := New(Dependencies{Auth: manager, Profiles: profiles, ConfigRoot: configRoot, Now: func() time.Time { return now }})
	loginResponse := performJSON(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"a-strong-password"}`, nil, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%q", loginResponse.Code, loginResponse.Body.String())
	}
	var session sessionResponse
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return profileHarness{handler: handler, service: profiles, repo: repo, configRoot: configRoot, cookie: loginResponse.Result().Cookies()[0], csrf: session.CSRFToken}
}

func (h profileHarness) request(method, path, body string) *httptest.ResponseRecorder {
	return performJSON(h.handler, method, path, body, h.cookie, h.csrf)
}

func TestProfileCreateListRenameAndPathOwnership(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%q", created.Code, created.Body.String())
	}
	var item ProfileResponse
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	wantDirectory := filepath.Join(h.configRoot, "primary")
	if item.ID == "" || item.DraftRevision != 1 || item.DirectoryHint != wantDirectory || item.DraftReadOnly || item.DraftError != "" || !strings.Contains(item.CheckCommand, `it'"'"'s/primary`) || strings.Contains(created.Body.String(), "data-dir") {
		t.Fatalf("created=%#v body=%q", item, created.Body.String())
	}
	duplicate := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"重复","slug":"primary"}`)
	assertAPIError(t, duplicate, http.StatusConflict, "duplicate_slug")

	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(h.configRoot, "occupied")); err != nil {
		t.Fatal(err)
	}
	occupied := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"占用","slug":"occupied"}`)
	assertAPIError(t, occupied, http.StatusConflict, "slug_path_occupied")

	second := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"第二个","slug":"secondary"}`)
	if second.Code != http.StatusCreated {
		t.Fatalf("second status=%d body=%q", second.Code, second.Body.String())
	}
	listed := h.request(http.MethodGet, "/api/v1/profiles", "")
	var items []ProfileResponse
	if err := json.Unmarshal(listed.Body.Bytes(), &items); err != nil || len(items) != 2 || items[0].ID != item.ID {
		t.Fatalf("list status=%d items=%#v err=%v", listed.Code, items, err)
	}
	renamed := h.request(http.MethodPatch, "/api/v1/profiles/"+item.ID, `{"name":"新名称"}`)
	var changed ProfileResponse
	if err := json.Unmarshal(renamed.Body.Bytes(), &changed); err != nil || renamed.Code != http.StatusOK || changed.Name != "新名称" || changed.Slug != "primary" {
		t.Fatalf("rename status=%d body=%q changed=%#v err=%v", renamed.Code, renamed.Body.String(), changed, err)
	}
}

func TestDraftETagConflictAndValidation(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)
	got := h.request(http.MethodGet, "/api/v1/profiles/"+item.ID+"/draft", "")
	if got.Code != http.StatusOK || got.Header().Get("ETag") != `"draft-1"` {
		t.Fatalf("get status=%d etag=%q body=%q", got.Code, got.Header().Get("ETag"), got.Body.String())
	}
	newSaveRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+item.ID+"/draft", strings.NewReader(`{"config":{"log":{"level":"debug"}},"editorBuffers":{}}`))
		request.Header.Set("X-CSRF-Token", h.csrf)
		request.Header.Set("If-Match", `"draft-1"`)
		request.AddCookie(h.cookie)
		return request
	}
	saved := httptest.NewRecorder()
	h.handler.ServeHTTP(saved, newSaveRequest())
	if saved.Code != http.StatusOK || saved.Header().Get("ETag") != `"draft-2"` {
		t.Fatalf("save status=%d etag=%q body=%q", saved.Code, saved.Header().Get("ETag"), saved.Body.String())
	}
	stale := httptest.NewRecorder()
	h.handler.ServeHTTP(stale, newSaveRequest().Clone(context.Background()))
	if stale.Code != http.StatusConflict || stale.Body.String() != "{\"error\":{\"code\":\"draft_conflict\",\"message\":\"草稿已被其他页面更新\",\"currentRevision\":2}}\n" {
		t.Fatalf("stale status=%d body=%q", stale.Code, stale.Body.String())
	}
	missing := h.request(http.MethodPut, "/api/v1/profiles/"+item.ID+"/draft", `{"config":{},"editorBuffers":{}}`)
	assertAPIError(t, missing, http.StatusPreconditionRequired, "revision_required")
}

func TestProfileRejectsTraversalAbsoluteSlugAndUnknownFields(t *testing.T) {
	h := newProfileHarness(t)
	assertAPIError(t, h.request(http.MethodGet, "/api/v1/profiles/%2e%2e%2f%2e%2e%2fetc%2fpasswd", ""), http.StatusNotFound, "not_found")
	assertAPIError(t, h.request(http.MethodPost, "/api/v1/profiles", `{"name":"bad","slug":"/tmp/bad"}`), http.StatusUnprocessableEntity, "validation_failed")
	assertAPIError(t, h.request(http.MethodPost, "/api/v1/profiles", `{"name":"bad","slug":"bad","path":"/tmp"}`), http.StatusBadRequest, "invalid_request")
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)
	assertAPIError(t, h.request(http.MethodPatch, "/api/v1/profiles/"+item.ID, `{"name":"bad","directory":"/tmp"}`), http.StatusBadRequest, "invalid_request")
}

func TestProfileRoutesRequireAuthenticationAndCSRF(t *testing.T) {
	h := newProfileHarness(t)
	unauthenticated := performJSON(h.handler, http.MethodGet, "/api/v1/profiles", "", nil, "")
	assertAPIError(t, unauthenticated, http.StatusUnauthorized, "unauthorized")
	missingCSRF := performJSON(h.handler, http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`, h.cookie, "")
	assertAPIError(t, missingCSRF, http.StatusForbidden, "invalid_csrf")
}

func TestDraftModuleByteLimits(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)
	save := func(revision uint64, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+item.ID+"/draft", strings.NewReader(body))
		request.Header.Set("X-CSRF-Token", h.csrf)
		request.Header.Set("If-Match", draftETag(revision))
		request.AddCookie(h.cookie)
		response := httptest.NewRecorder()
		h.handler.ServeHTTP(response, request)
		return response
	}
	exactValue := `"` + strings.Repeat("x", maxModuleBytes-2) + `"`
	exact := save(1, `{"config":{"large":`+exactValue+`},"editorBuffers":{}}`)
	if exact.Code != http.StatusOK || exact.Header().Get("ETag") != `"draft-2"` {
		t.Fatalf("exact config status=%d body=%q", exact.Code, exact.Body.String())
	}
	overValue := `"` + strings.Repeat("sensitive", (maxModuleBytes/9)+1) + `"`
	over := save(2, `{"config":{"safe-key":`+overValue+`},"editorBuffers":{}}`)
	if over.Code != http.StatusRequestEntityTooLarge || !strings.Contains(over.Body.String(), `"code":"module_too_large"`) || !strings.Contains(over.Body.String(), `"key":"safe-key"`) || strings.Contains(over.Body.String(), "sensitive") {
		t.Fatalf("over config status=%d body=%q", over.Code, over.Body.String())
	}
	exactBuffer := strings.Repeat("x", maxModuleBytes)
	bufferBody, err := json.Marshal(draftRequest{Config: json.RawMessage(`{}`), EditorBuffers: map[string]profile.EditorBuffer{"log": {Text: exactBuffer}}})
	if err != nil {
		t.Fatal(err)
	}
	buffer := save(2, string(bufferBody))
	if buffer.Code != http.StatusOK || buffer.Header().Get("ETag") != `"draft-3"` {
		t.Fatalf("exact buffer status=%d body=%q", buffer.Code, buffer.Body.String())
	}
	overBufferBody, err := json.Marshal(draftRequest{Config: json.RawMessage(`{}`), EditorBuffers: map[string]profile.EditorBuffer{"route": {Text: strings.Repeat("y", maxModuleBytes+1)}}})
	if err != nil {
		t.Fatal(err)
	}
	overBuffer := save(3, string(overBufferBody))
	assertAPIError(t, overBuffer, http.StatusRequestEntityTooLarge, "module_too_large")
}

func TestDraftModuleLimitCannotBeBypassedWithDuplicateKeys(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)
	save := func(revision uint64, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+item.ID+"/draft", strings.NewReader(body))
		request.Header.Set("X-CSRF-Token", h.csrf)
		request.Header.Set("If-Match", draftETag(revision))
		request.AddCookie(h.cookie)
		response := httptest.NewRecorder()
		h.handler.ServeHTTP(response, request)
		return response
	}
	overValue := `"` + strings.Repeat("secret", (maxModuleBytes/6)+1) + `"`
	bypass := save(1, `{"config":{"duplicate":`+overValue+`,"duplicate":0},"editorBuffers":{}}`)
	if bypass.Code != http.StatusRequestEntityTooLarge || strings.Contains(bypass.Body.String(), "secret") {
		t.Fatalf("duplicate bypass status=%d bodyBytes=%d", bypass.Code, bypass.Body.Len())
	}
	duplicate := save(1, `{"config":{"duplicate":0,"duplicate":1},"editorBuffers":{}}`)
	assertAPIError(t, duplicate, http.StatusUnprocessableEntity, "validation_failed")

	multibyteContent := strings.Repeat("界", 333_332) + "ab"
	if len([]byte(multibyteContent))+2 != maxModuleBytes {
		t.Fatalf("test value bytes=%d", len([]byte(multibyteContent))+2)
	}
	exact := save(1, `{"config":{"多字节":"`+multibyteContent+`"},"editorBuffers":{}}`)
	if exact.Code != http.StatusOK || exact.Header().Get("ETag") != `"draft-2"` {
		t.Fatalf("exact multibyte status=%d etag=%q bodyBytes=%d", exact.Code, exact.Header().Get("ETag"), exact.Body.Len())
	}
	over := save(2, `{"config":{"多字节":"`+multibyteContent+`c"},"editorBuffers":{}}`)
	if over.Code != http.StatusRequestEntityTooLarge || strings.Contains(over.Body.String(), "界") {
		t.Fatalf("over multibyte status=%d body=%q", over.Code, over.Body.String())
	}
}

func TestProfileDynamicRouteMethodMatrix(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)

	profileDelete := h.request(http.MethodDelete, "/api/v1/profiles/"+item.ID, "")
	assertAPIError(t, profileDelete, http.StatusMethodNotAllowed, "method_not_allowed")
	if profileDelete.Header().Get("Allow") != "PATCH" {
		t.Fatalf("profile Allow=%q", profileDelete.Header().Get("Allow"))
	}
	draftPost := h.request(http.MethodPost, "/api/v1/profiles/"+item.ID+"/draft", "")
	assertAPIError(t, draftPost, http.StatusMethodNotAllowed, "method_not_allowed")
	if draftPost.Header().Get("Allow") != "GET, PUT" {
		t.Fatalf("draft Allow=%q", draftPost.Header().Get("Allow"))
	}
	unknownID := strings.Repeat("a", 32)
	assertAPIError(t, h.request(http.MethodDelete, "/api/v1/profiles/"+unknownID, ""), http.StatusNotFound, "not_found")
	assertAPIError(t, h.request(http.MethodDelete, "/api/v1/profiles/not-an-id", ""), http.StatusNotFound, "not_found")
	assertAPIError(t, h.request(http.MethodGet, "/api/v1/profiles/"+item.ID+"/unknown", ""), http.StatusNotFound, "not_found")
}

func TestDraftCorruptIsReadOnlyAndUnchanged(t *testing.T) {
	h := newProfileHarness(t)
	created := h.request(http.MethodPost, "/api/v1/profiles", `{"name":"主配置","slug":"primary"}`)
	var item ProfileResponse
	_ = json.Unmarshal(created.Body.Bytes(), &item)
	path := h.repo.DraftPath(item.ID)
	want := []byte(`{"secret":"must-not-leak"`)
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	get := h.request(http.MethodGet, "/api/v1/profiles/"+item.ID+"/draft", "")
	assertAPIError(t, get, http.StatusConflict, "draft_corrupt")
	listed := h.request(http.MethodGet, "/api/v1/profiles", "")
	var items []ProfileResponse
	if err := json.Unmarshal(listed.Body.Bytes(), &items); err != nil || len(items) != 1 || !items[0].DraftReadOnly || items[0].DraftError != "草稿 JSON 损坏，需要管理员在服务器修复" {
		t.Fatalf("list status=%d body=%q items=%#v err=%v", listed.Code, listed.Body.String(), items, err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+item.ID+"/draft", strings.NewReader(`{"config":{},"editorBuffers":{}}`))
	request.Header.Set("X-CSRF-Token", h.csrf)
	request.Header.Set("If-Match", `"draft-1"`)
	request.AddCookie(h.cookie)
	put := httptest.NewRecorder()
	h.handler.ServeHTTP(put, request)
	assertAPIError(t, put, http.StatusConflict, "draft_corrupt")
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, want) {
		t.Fatalf("draft changed: got=%q want=%q err=%v", after, want, err)
	}
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != status || body.Error.Code != code {
		t.Fatalf("status=%d body=%q decoded=%#v err=%v", response.Code, response.Body.String(), body, err)
	}
}
