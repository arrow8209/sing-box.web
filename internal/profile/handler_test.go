package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/auth"
	"singbox-web/internal/httpapi"
)

type profileAPISession struct {
	cookie *http.Cookie
	csrf   string
}

type allowDeleteGuard struct{}

func (allowDeleteGuard) GuardDelete(context.Context, string, bool) error { return nil }

func TestDraftAPIRequiresIfMatchAndReturnsLatestRevision(t *testing.T) {
	handler, session := newAuthenticatedProfileAPI(t)
	profileID := createProfile(t, handler, session, `{"name":"家庭出口"}`)

	get := profileRequest(t, handler, session, http.MethodGet, "/api/v1/profiles/"+profileID+"/draft", "", nil)
	if get.Code != http.StatusOK || get.Header().Get("ETag") != `"rev-1"` {
		t.Fatalf("get=%d etag=%q body=%s", get.Code, get.Header().Get("ETag"), get.Body.String())
	}

	first := profileRequest(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", validDraftBody(), map[string]string{"If-Match": `"rev-1"`})
	if first.Code != http.StatusOK || first.Header().Get("ETag") != `"rev-2"` {
		t.Fatalf("first=%d etag=%q body=%s", first.Code, first.Header().Get("ETag"), first.Body.String())
	}

	stale := profileRequest(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", validDraftBody(), map[string]string{"If-Match": `"rev-1"`})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale=%d body=%s", stale.Code, stale.Body.String())
	}
	assertProfileJSON(t, stale.Body.Bytes(), "error.code", "draft_revision_conflict")
	assertProfileJSON(t, stale.Body.Bytes(), "error.latestRevision", float64(2))
}

func TestDraftSaveHookRunsOnlyAfterSuccessfulPut(t *testing.T) {
	service, _ := newTestService(t)
	meta, _, err := service.Create(context.Background(), CreateInput{Name: "自动重载"})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	handler := NewHandler(service, func(_ context.Context, profileID string, revision uint64) *DraftSaveOutcome {
		calls++
		if profileID != meta.ID || revision != 2 {
			t.Errorf("profileID=%q revision=%d", profileID, revision)
		}
		return &DraftSaveOutcome{Attempted: true, Applied: true, Reloaded: false, Message: "配置已自动应用"}
	})
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/profiles/"+meta.ID+"/draft", nil))
	if get.Code != http.StatusOK || calls != 0 {
		t.Fatalf("get=%d calls=%d body=%s", get.Code, calls, get.Body.String())
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+meta.ID+"/draft", strings.NewReader(validDraftBody()))
	request.Header.Set("If-Match", `"rev-1"`)
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, request)
	if put.Code != http.StatusOK || calls != 1 {
		t.Fatalf("put=%d calls=%d body=%s", put.Code, calls, put.Body.String())
	}
	assertProfileJSON(t, put.Body.Bytes(), "autoReload.message", "配置已自动应用")
}

func TestProfileCreateRejectsDirectoryFields(t *testing.T) {
	handler, session := newAuthenticatedProfileAPI(t)
	for _, body := range []string{`{"name":"x","configDir":"x"}`, `{"name":"x","path":"../x"}`} {
		response := profileRequest(t, handler, session, http.MethodPost, "/api/v1/profiles", body, nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestProfileCRUDRequiresAuthenticationAndCSRF(t *testing.T) {
	handler, session := newAuthenticatedProfileAPIWithDeleteGuard(t, allowDeleteGuard{})

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", unauthenticated.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", bytes.NewBufferString(`{"name":"未带 CSRF"}`))
	request.AddCookie(session.cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing csrf=%d", response.Code)
	}

	profileID := createProfile(t, handler, session, `{"name":"初始名称"}`)
	renamed := profileRequest(t, handler, session, http.MethodPatch, "/api/v1/profiles/"+profileID, `{"name":"修改后"}`, nil)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename=%d body=%s", renamed.Code, renamed.Body.String())
	}
	assertProfileJSON(t, renamed.Body.Bytes(), "name", "修改后")

	deleted := profileRequest(t, handler, session, http.MethodDelete, "/api/v1/profiles/"+profileID, `{"confirmName":"修改后","stopCurrent":false}`, nil)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete=%d body=%s", deleted.Code, deleted.Body.String())
	}
}

func TestProfileCopyCreatesIndependentDraft(t *testing.T) {
	handler, session := newAuthenticatedProfileAPI(t)
	profileID := createProfile(t, handler, session, `{"name":"家庭出口"}`)
	updated := profileRequest(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", `{"buffers":{"log":"{\"level\":\"debug\"}"},"editorState":{"inbounds":{"disabledRules":[1]}},"bindings":[]}`, map[string]string{"If-Match": `"rev-1"`})
	if updated.Code != http.StatusOK {
		t.Fatalf("update=%d body=%s", updated.Code, updated.Body.String())
	}

	first := profileRequest(t, handler, session, http.MethodPost, "/api/v1/profiles/"+profileID+"/copy", "", nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("copy=%d body=%s", first.Code, first.Body.String())
	}
	var copied Meta
	if err := json.Unmarshal(first.Body.Bytes(), &copied); err != nil {
		t.Fatal(err)
	}
	if copied.ID == "" || copied.ID == profileID || copied.Name != "家庭出口 副本" || copied.DraftRevision != 1 || !copied.NeedsApply {
		t.Fatalf("copied=%+v", copied)
	}
	draftResponse := profileRequest(t, handler, session, http.MethodGet, "/api/v1/profiles/"+copied.ID+"/draft", "", nil)
	if draftResponse.Code != http.StatusOK || draftResponse.Header().Get("ETag") != `"rev-1"` {
		t.Fatalf("draft=%d etag=%q body=%s", draftResponse.Code, draftResponse.Header().Get("ETag"), draftResponse.Body.String())
	}
	var draft Draft
	if err := json.Unmarshal(draftResponse.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.ProfileID != copied.ID || draft.Revision != 1 || draft.Buffers["log"] != `{"level":"debug"}` || !strings.Contains(string(draft.EditorState["inbounds"]), "disabledRules") {
		t.Fatalf("draft=%+v", draft)
	}

	second := profileRequest(t, handler, session, http.MethodPost, "/api/v1/profiles/"+profileID+"/copy", "", nil)
	if second.Code != http.StatusCreated {
		t.Fatalf("second copy=%d body=%s", second.Code, second.Body.String())
	}
	assertProfileJSON(t, second.Body.Bytes(), "name", "家庭出口 副本 2")
}

func TestProfileDeleteRequiresCoreDecisionByDefault(t *testing.T) {
	handler, session := newAuthenticatedProfileAPI(t)
	profileID := createProfile(t, handler, session, `{"name":"活动档案"}`)

	response := profileRequest(t, handler, session, http.MethodDelete, "/api/v1/profiles/"+profileID, `{"confirmName":"活动档案","stopCurrent":false}`, nil)
	if response.Code != http.StatusConflict {
		t.Fatalf("delete=%d body=%s", response.Code, response.Body.String())
	}
	assertProfileJSON(t, response.Body.Bytes(), "error.code", "current_profile_requires_core_decision")
}

func TestDraftAPIRejectsForgedEnvelopeAndOversizedBody(t *testing.T) {
	handler, session := newAuthenticatedProfileAPI(t)
	profileID := createProfile(t, handler, session, `{"name":"草稿"}`)

	forged := profileRequest(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", `{"config":{},"buffers":{},"bindings":[]}`, map[string]string{"If-Match": `"rev-1"`})
	if forged.Code != http.StatusBadRequest {
		t.Fatalf("forged=%d body=%s", forged.Code, forged.Body.String())
	}

	over := `{"buffers":{"x":"` + string(bytes.Repeat([]byte("a"), (4<<20)+1)) + `"},"bindings":[]}`
	tooLarge := profileRequest(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", over, map[string]string{"If-Match": `"rev-1"`})
	if tooLarge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large=%d body=%s", tooLarge.Code, tooLarge.Body.String())
	}
}

func newAuthenticatedProfileAPI(t *testing.T) (http.Handler, profileAPISession) {
	t.Helper()
	return newAuthenticatedProfileAPIWithDeleteGuard(t, nil)
}

func newAuthenticatedProfileAPIWithDeleteGuard(t *testing.T, guard DeleteGuard) (http.Handler, profileAPISession) {
	t.Helper()
	service, layout := newTestService(t)
	if guard != nil {
		service.SetDeleteGuard(guard)
	}
	store := auth.NewStore(filepath.Join(layout.DataDir, "auth.json"), nil)
	if err := store.SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	authHandler := auth.NewHandlerWithDependencies(store, auth.NewSessionManager(nil), auth.NewLoginLimiter(5, 1), testClock)
	handler := httpapi.NewRouter(httpapi.Dependencies{
		ProfileHandler: NewHandler(service),
		Authenticate:   authHandler.Middleware,
		RequireCSRF:    auth.RequireCSRF,
	})
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"password":"correct horse battery staple"}`))
	loginResponse := httptest.NewRecorder()
	authHandler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var payload struct {
		CSRFToken string `json:"csrfToken"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || payload.CSRFToken == "" {
		t.Fatalf("cookies=%d csrf=%q", len(cookies), payload.CSRFToken)
	}
	return handler, profileAPISession{cookie: cookies[0], csrf: payload.CSRFToken}
}

func createProfile(t *testing.T, handler http.Handler, session profileAPISession, body string) string {
	t.Helper()
	response := profileRequest(t, handler, session, http.MethodPost, "/api/v1/profiles", body, nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", response.Code, response.Body.String())
	}
	var meta Meta
	if err := json.Unmarshal(response.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" {
		t.Fatalf("missing profile id: %s", response.Body.String())
	}
	return meta.ID
}

func profileRequest(t *testing.T, handler http.Handler, session profileAPISession, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.AddCookie(session.cookie)
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		request.Header.Set("X-CSRF-Token", session.csrf)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func validDraftBody() string { return `{"buffers":{},"bindings":[]}` }

func assertProfileJSON(t *testing.T, body []byte, path string, want any) {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	var value any = document
	for _, key := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("path %s does not resolve in %s", path, body)
		}
		value = object[key]
	}
	if value != want {
		t.Fatalf("path %s = %#v, want %#v", path, value, want)
	}
}
