package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/apierror"
)

func TestReleaseRateLimitIsVisibleInLookupAndDownloadStatus(t *testing.T) {
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		result := response(http.StatusForbidden, `{"message":"private upstream details"}`)
		result.Header.Set("X-RateLimit-Remaining", "0")
		result.Header.Set("X-RateLimit-Reset", "1893456000")
		return result, nil
	})
	service := &Service{
		releases: NewReleaseClient(ReleaseClientPolicyForTest(transport, "amd64")),
		active:   true,
		status:   Status{OperationID: "limited-update"},
	}
	recorder := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/core/releases/latest", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != time.Unix(1893456000, 0).UTC().Format(http.TimeFormat) {
		t.Fatalf("Retry-After=%q", got)
	}
	var body struct {
		Error apierror.Error `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "release_rate_limited" || !strings.Contains(body.Error.Message, "GitHub API 请求次数已达上限，请在 ") {
		t.Fatalf("error=%+v", body.Error)
	}
	if strings.Contains(recorder.Body.String(), "private upstream details") {
		t.Fatal("upstream response body leaked into API response")
	}

	ctx, cancel := context.WithCancel(context.Background())
	service.run("limited-update", ctx, cancel, UpdateConfirmation{ConfirmedVersion: "1.0.0"})
	status := service.Status()
	if status.Stage != "failed" || status.FinishedAt == nil || status.Message != body.Error.Message {
		t.Fatalf("download status=%+v", status)
	}
}

func TestReleaseLookupKeepsOtherFailuresGeneric(t *testing.T) {
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{"message":"private upstream details"}`), nil
	})
	service := &Service{releases: NewReleaseClient(ReleaseClientPolicyForTest(transport, "amd64"))}
	recorder := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/core/releases/latest", nil))
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "release_lookup_failed") || strings.Contains(recorder.Body.String(), "private upstream details") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
