package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/core"
)

type metricsCoreStatus struct {
	mu     sync.Mutex
	status core.Status
}

func (s *metricsCoreStatus) Status(context.Context) core.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *metricsCoreStatus) set(value core.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = value
}

func TestMetricsReadsCoreCountersWithoutExposingConnections(t *testing.T) {
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/connections" || r.Header.Get("Authorization") != "Bearer metrics-controller-only" {
			t.Error("incorrect metrics upstream request")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"memory":33554432,"uploadTotal":4096,"downloadTotal":8192,"connections":[{"id":"first","metadata":{"host":"private.example.test"}},{"id":"second"}]}`))
	}))
	defer controller.Close()
	startedAt := time.Unix(1000, 0).UTC()
	source := &metricsCoreStatus{status: core.Status{State: core.ProcessRunning, PID: 42, StartedAt: &startedAt}}
	layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "metrics-controller-only")
	handler := NewHandler(NewService(layout, source))
	before := time.Now()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsPath, nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var snapshot MetricsSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Memory != 33554432 || snapshot.UploadTotal != 4096 || snapshot.DownloadTotal != 8192 || snapshot.ConnectionCount != 2 {
		t.Fatalf("metrics=%+v", snapshot)
	}
	if snapshot.RuntimeKey != "42:1970-01-01T00:16:40Z" || snapshot.ObservedAt.Before(before) || snapshot.ObservedAt.After(time.Now()) {
		t.Fatalf("sample identity/time=%+v", snapshot)
	}
	for _, hidden := range []string{"metrics-controller-only", "private.example.test", "first", "metadata"} {
		if strings.Contains(response.Body.String(), hidden) {
			t.Fatalf("aggregate metrics exposed %q", hidden)
		}
	}
}

func TestMetricsDistinguishesRealZeroFromMissingOrInvalidData(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       int
	}{
		{name: "idle", body: `{"memory":1024,"uploadTotal":0,"downloadTotal":0,"connections":[]}`, want: 200},
		{name: "null empty connections", body: `{"memory":1024,"uploadTotal":0,"downloadTotal":0,"connections":null}`, want: 200},
		{name: "missing memory", body: `{"uploadTotal":0,"downloadTotal":0,"connections":[]}`, want: 502},
		{name: "null memory", body: `{"memory":null,"uploadTotal":0,"downloadTotal":0,"connections":[]}`, want: 502},
		{name: "missing upload", body: `{"memory":1024,"downloadTotal":0,"connections":[]}`, want: 502},
		{name: "missing download", body: `{"memory":1024,"uploadTotal":0,"connections":[]}`, want: 502},
		{name: "missing connections", body: `{"memory":1024,"uploadTotal":0,"downloadTotal":0}`, want: 502},
		{name: "invalid connections", body: `{"memory":1024,"uploadTotal":0,"downloadTotal":0,"connections":{}}`, want: 502},
		{name: "negative counter", body: `{"memory":1024,"uploadTotal":-1,"downloadTotal":0,"connections":[]}`, want: 502},
		{name: "fractional counter", body: `{"memory":1024,"uploadTotal":1.5,"downloadTotal":0,"connections":[]}`, want: 502},
		{name: "invalid JSON", body: `{"memory":`, want: 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(test.body)) }))
			defer controller.Close()
			layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "")
			handler := NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsPath, nil))
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.want == 200 {
				var snapshot MetricsSnapshot
				if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil || snapshot.UploadTotal != 0 || snapshot.DownloadTotal != 0 || snapshot.ConnectionCount != 0 {
					t.Fatalf("idle metrics=%+v error=%v", snapshot, err)
				}
			} else if !strings.Contains(response.Body.String(), "clash_api_unavailable") {
				t.Fatalf("missing unavailable error: %s", response.Body.String())
			}
		})
	}
}

func TestMetricsRejectsSamplesAcrossCoreStopOrRestart(t *testing.T) {
	for _, state := range []core.ProcessState{core.ProcessStopped, core.ProcessRunning} {
		t.Run(string(state), func(t *testing.T) {
			source := &metricsCoreStatus{status: core.Status{State: core.ProcessRunning, PID: 42}}
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				source.set(core.Status{State: state, PID: 43})
				_, _ = w.Write([]byte(`{"memory":1024,"uploadTotal":100,"downloadTotal":200,"connections":[]}`))
			}))
			defer controller.Close()
			layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "")
			handler := NewHandler(NewService(layout, source))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsPath, nil))
			if response.Code != http.StatusConflict && response.Code != http.StatusBadGateway {
				t.Fatalf("accepted sample from old core: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestMetricsDoesNotReadStoppedCoreOrAcceptWrites(t *testing.T) {
	layout := writeControllerConfig(t, "127.0.0.1:50017", "")
	client := &recordingClient{}
	handler := NewHandler(NewServiceWithClient(layout, fixedCoreStatus{state: core.ProcessStopped}, client))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsPath, nil))
	if response.Code != http.StatusConflict || client.calls != 0 {
		t.Fatalf("status=%d upstream calls=%d", response.Code, client.calls)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, metricsPath, strings.NewReader(`{}`)))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET" || client.calls != 0 {
		t.Fatal("metrics must be read-only")
	}
}
