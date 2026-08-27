package application

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"singbox-web/internal/apierror"
)

func TestApplyEndpointIsExplicitlyNotImplemented(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/0123456789abcdef0123456789abcdef/apply", nil)
	NewHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Error apierror.Error `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "application_not_implemented" || response.Error.Message != "应用逻辑尚未实现" {
		t.Fatalf("error=%+v", response.Error)
	}
}
