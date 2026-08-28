package application

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"singbox-web/internal/apierror"
	"singbox-web/internal/rules"
)

func TestApplyEndpointReportsUnavailableCoordinator(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/0123456789abcdef0123456789abcdef/apply", nil)
	NewHandler(nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "application_unavailable") || !strings.Contains(body, "应用服务不可用") {
		t.Fatalf("body=%s", body)
	}
}

func TestApplyEndpointReportsUnavailableRuleResource(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Handler{}).writeApplyError(recorder, rules.ErrResourceNotDownloaded)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Error apierror.Error `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "rule_resource_unavailable" || !strings.Contains(response.Error.Message, "更新网络规则") {
		t.Fatalf("error=%+v", response.Error)
	}
}

func TestApplyEndpointValidatesMethodPathAndRevision(t *testing.T) {
	handler := &Handler{coordinator: &Coordinator{}}
	tests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/api/v1/profiles/0123456789abcdef0123456789abcdef/apply", "", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/v1/profiles/not-an-id/apply", `{}`, http.StatusNotFound},
		{http.MethodPost, "/api/v1/profiles/0123456789abcdef0123456789abcdef/apply", `{"revision":0}`, http.StatusBadRequest},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		handler.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
}
