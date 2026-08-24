package rules

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestManualResourceAPIStoresContentAndReturnsDetail(t *testing.T) {
	service, _ := newRuleService(t, nil)
	api := NewHandler(service)
	created := httptest.NewRecorder()
	api.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources", bytes.NewBufferString(`{"sourceType":"manual","name":"私有网络","content":{"version":3,"rules":[{"ip_is_private":true}]}}`)))
	if created.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", created.Code, created.Body.String())
	}
	var resource ResourceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &resource); err != nil {
		t.Fatal(err)
	}
	detail := httptest.NewRecorder()
	api.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/rule-resources/"+resource.ID, nil))
	if detail.Code != http.StatusOK || !bytes.Contains(detail.Body.Bytes(), []byte(`"ip_is_private"`)) {
		t.Fatalf("status=%d body=%s", detail.Code, detail.Body.String())
	}
}

func TestCatalogAPIRefreshesPreviewsImportsAndRejectsDuplicates(t *testing.T) {
	service, layout := newRuleService(t, nil)
	service.SetDownloader(catalogRuleDownloader{})
	document := catalogRemoteDocument{
		GeoSite: geositeBaseURL,
		GeoIP:   geoipBaseURL,
		List:    []catalogRemoteItem{{Name: "cn", Type: "geosite", Count: 10}},
	}
	catalog := NewCatalogService(layout.RuleCatalogFile, layout, service, fixedCatalogFetcher{document: document, digest: strings.Repeat("a", 64)}, nil)
	api := NewHandler(service, catalog)

	refresh := httptest.NewRecorder()
	api.ServeHTTP(refresh, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources/catalog/refresh", nil))
	if refresh.Code != http.StatusOK || !bytes.Contains(refresh.Body.Bytes(), []byte(`"total":1`)) || !bytes.Contains(refresh.Body.Bytes(), []byte(`"geosite":"`+geositeBaseURL+`"`)) {
		t.Fatalf("refresh=%d %s", refresh.Code, refresh.Body.String())
	}

	preview := httptest.NewRecorder()
	api.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/api/v1/rule-resources/catalog/preview?key=geosite%3Acn", nil))
	if preview.Code != http.StatusOK || !bytes.Contains(preview.Body.Bytes(), []byte(`"ruleCount":2`)) {
		t.Fatalf("preview=%d %s", preview.Code, preview.Body.String())
	}

	source := httptest.NewRecorder()
	api.ServeHTTP(source, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources/catalog/import", bytes.NewBufferString(`{"key":"geosite:cn","format":"source","updateInterval":"24h"}`)))
	if source.Code != http.StatusBadRequest || !bytes.Contains(source.Body.Bytes(), []byte("网络规则只能使用 binary 格式")) {
		t.Fatalf("source import=%d %s", source.Code, source.Body.String())
	}

	input := `{"key":"geosite:cn","format":"binary","updateInterval":"24h"}`
	imported := httptest.NewRecorder()
	api.ServeHTTP(imported, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources/catalog/import", bytes.NewBufferString(input)))
	if imported.Code != http.StatusCreated || !bytes.Contains(imported.Body.Bytes(), []byte(geositeBaseURL+"cn.srs")) || !bytes.Contains(imported.Body.Bytes(), []byte(`"catalogKey":"geosite:cn"`)) || !bytes.Contains(imported.Body.Bytes(), []byte(`"ruleCount":10`)) {
		t.Fatalf("import=%d %s", imported.Code, imported.Body.String())
	}

	duplicate := httptest.NewRecorder()
	api.ServeHTTP(duplicate, httptest.NewRequest(http.MethodPost, "/api/v1/rule-resources/catalog/import", bytes.NewBufferString(input)))
	if duplicate.Code != http.StatusConflict || !bytes.Contains(duplicate.Body.Bytes(), []byte(`"code":"rule_catalog_already_imported"`)) {
		t.Fatalf("duplicate=%d %s", duplicate.Code, duplicate.Body.String())
	}
}
