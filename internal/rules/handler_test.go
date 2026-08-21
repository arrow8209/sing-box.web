package rules

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"singbox-web/internal/httpapi"
)

func TestCreateRejectsClientLocalPath(t *testing.T) {
	service, _ := newRuleService(t, nil)
	api := httpapi.NewRouter(httpapi.Dependencies{RuleResourceHandler: NewHandler(service), Authenticate: passthrough, RequireCSRF: passthrough})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources", bytes.NewBufferString(`{"name":"CN","url":"https://example.test/cn.srs","format":"binary","localPath":"/tmp/cn.srs"}`))
	api.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRuleResourcesRequireAuthenticationAndCSRFWrites(t *testing.T) {
	service, _ := newRuleService(t, nil)
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Session") != "ok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	csrf := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-CSRF") != "ok" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	api := httpapi.NewRouter(httpapi.Dependencies{RuleResourceHandler: NewHandler(service), Authenticate: auth, RequireCSRF: csrf})
	for _, test := range []struct {
		session, token string
		want           int
	}{{want: http.StatusUnauthorized}, {session: "ok", want: http.StatusForbidden}, {session: "ok", token: "ok", want: http.StatusCreated}} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources", bytes.NewBufferString(`{"name":"CN","url":"https://example.test/cn.srs","format":"binary"}`))
		request.Header.Set("X-Session", test.session)
		request.Header.Set("X-CSRF", test.token)
		api.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
		}
	}
}

func passthrough(next http.Handler) http.Handler { return next }

func TestDeleteReferenceAPIErrorIncludesProfiles(t *testing.T) {
	service, _ := newRuleService(t, map[string][]Reference{"11111111111111111111111111111111": {{ID: "p", Name: "家庭出口"}}})
	api := NewHandler(service)
	create := httptest.NewRecorder()
	api.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources", bytes.NewBufferString(`{"name":"CN","url":"https://example.test/cn.srs","format":"binary"}`)))
	if create.Code != http.StatusCreated {
		t.Fatal(create.Body.String())
	}
	var created ResourceResponse
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	deleted := httptest.NewRecorder()
	api.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/api/v1/rule-resources/"+created.ID, nil))
	if deleted.Code != http.StatusConflict || !bytes.Contains(deleted.Body.Bytes(), []byte("家庭出口")) {
		t.Fatalf("response=%d %s", deleted.Code, deleted.Body.String())
	}
}
