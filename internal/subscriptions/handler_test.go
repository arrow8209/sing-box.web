package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/httpapi"
	"singbox-web/internal/paths"
)

func TestSubscriptionAPIExposesSummariesAndCompleteDetail(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(layout, func() time.Time { return time.Unix(100, 0).UTC() }, bytes.NewReader(bytes.Repeat([]byte{0x44}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	api := httpapi.NewRouter(httpapi.Dependencies{
		SubscriptionHandler: NewHandler(service),
		Authenticate:        passthrough,
		RequireCSRF:         passthrough,
	})
	created := requestJSON(t, api, http.MethodPost, "/api/v1/subscriptions", `{"name":"E2E 节点","outbounds":[{"type":"trojan","tag":"新加坡","password":"secret"}]}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	var summary Summary
	if err := json.Unmarshal(created.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	list := requestJSON(t, api, http.MethodGet, "/api/v1/subscriptions", "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "secret") || strings.Contains(list.Body.String(), "outbounds") {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
	detail := requestJSON(t, api, http.MethodGet, "/api/v1/subscriptions/"+summary.ID, "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"password":"secret"`) {
		t.Fatalf("detail=%d %s", detail.Code, detail.Body.String())
	}
	replaced := requestJSON(t, api, http.MethodPut, "/api/v1/subscriptions/"+summary.ID, `{"name":"替换后","outbounds":[{"type":"vless","tag":"香港"}]}`)
	if replaced.Code != http.StatusOK || !strings.Contains(replaced.Body.String(), "替换后") {
		t.Fatalf("replace=%d %s", replaced.Code, replaced.Body.String())
	}
	deleted := requestJSON(t, api, http.MethodDelete, "/api/v1/subscriptions/"+summary.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete=%d %s", deleted.Code, deleted.Body.String())
	}
	missing := requestJSON(t, api, http.MethodGet, "/api/v1/subscriptions/"+summary.ID, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing=%d %s", missing.Code, missing.Body.String())
	}
}

func TestSubscriptionAPIRejectsUnknownFieldsAndInvalidIDs(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(layout, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service)
	unknown := requestJSON(t, handler, http.MethodPost, "/api/v1/subscriptions", `{"name":"x","outbounds":[{"type":"direct","tag":"direct"}],"url":"https://example.test/sub"}`)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown=%d %s", unknown.Code, unknown.Body.String())
	}
	invalidID := requestJSON(t, handler, http.MethodGet, "/api/v1/subscriptions/../escape", "")
	if invalidID.Code != http.StatusNotFound {
		t.Fatalf("invalid id=%d %s", invalidID.Code, invalidID.Body.String())
	}
}

func passthrough(next http.Handler) http.Handler { return next }

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
