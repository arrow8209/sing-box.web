package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"singbox-web/internal/core"
)

func delayRequest(t *testing.T, service *Service, tag string) *http.Request {
	t.Helper()
	key, err := service.proxyRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(ProxyDelayRequest{Tag: tag, RuntimeKey: key})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewRequest(http.MethodPost, proxyDelayPath, strings.NewReader(string(payload)))
}

func TestProxyDelayUsesCoreMeasurementAndEscapedName(t *testing.T) {
	const tag = "🇯🇵 节点 /?#% 延迟"
	for _, test := range []struct {
		name, body, status string
		upstream, want     int
	}{
		{"success", `{"delay":237}`, "ok", 200, 200},
		{"timeout", `{"message":"Timeout"}`, "timeout", 504, 200},
		{"connection failure", `{"message":"private detail"}`, "failed", 503, 200},
		{"missing node", `{}`, "", 404, 400},
		{"controller failure", `{}`, "", 500, 502},
		{"controller auth", `{}`, "", 401, 502},
		{"invalid JSON", `{`, "", 200, 502},
		{"missing delay", `{}`, "", 200, 502},
		{"zero delay", `{"delay":0}`, "", 200, 502},
		{"negative delay", `{"delay":-1}`, "", 200, 502},
		{"fractional delay", `{"delay":1.5}`, "", 200, 502},
		{"overflow delay", `{"delay":65536}`, "", 200, 502},
		{"oversized response", strings.Repeat(" ", (16<<10)+1), "", 200, 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			var tests int
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer test-controller-only" {
					t.Error("invalid upstream authentication/method")
				}
				if r.URL.EscapedPath() == "/proxies/"+url.PathEscape(tag) {
					_, _ = w.Write([]byte(`{"type":"Trojan"}`))
					return
				}
				if r.URL.EscapedPath() != "/proxies/"+url.PathEscape(tag)+"/delay" || len(r.URL.Query()) != 2 || r.URL.Query().Get("url") != "https://www.gstatic.com/generate_204" || r.URL.Query().Get("timeout") != "5000" {
					t.Errorf("invalid delay path or query: %s", r.URL)
				}
				tests++
				w.WriteHeader(test.upstream)
				_, _ = w.Write([]byte(test.body))
			}))
			defer controller.Close()
			service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "test-controller-only"), fixedCoreStatus{state: core.ProcessRunning})
			response := httptest.NewRecorder()
			before := time.Now()
			NewHandler(service).ServeHTTP(response, delayRequest(t, service, tag))
			if response.Code != test.want || tests != 1 {
				t.Fatalf("status=%d tests=%d body=%s", response.Code, tests, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "private detail") || strings.Contains(response.Body.String(), "test-controller-only") {
				t.Fatal("leaked upstream details")
			}
			if test.want != 200 {
				return
			}
			var result ProxyDelayResult
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != test.status || result.Tag != tag || result.RuntimeKey == "" || result.TestedAt.Before(before) || result.TestedAt.After(time.Now()) {
				t.Fatalf("result=%+v", result)
			}
			if test.status == "ok" && result.Delay != 237 {
				t.Fatalf("not the core's delay: %d", result.Delay)
			}
			if test.status != "ok" && strings.Contains(response.Body.String(), `"delay":`) {
				t.Fatal("failure reported as a numeric delay")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("measurement is cacheable")
			}
		})
	}
}

func TestProxyDelayRejectsInvalidRequestsAndNonLeafOutbounds(t *testing.T) {
	for _, kind := range []string{"Reject", "Block", "DNS", "Selector", "URLTest", "Fallback", ""} {
		t.Run(kind, func(t *testing.T) {
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/delay") {
					t.Error("unsupported outbound was tested")
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"type": kind})
			}))
			defer controller.Close()
			service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), ""), fixedCoreStatus{state: core.ProcessRunning})
			response := httptest.NewRecorder()
			NewHandler(service).ServeHTTP(response, delayRequest(t, service, "node"))
			if response.Code != 400 || !strings.Contains(response.Body.String(), "proxy_delay_unsupported") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	client := &recordingClient{}
	service := NewServiceWithClient(writeControllerConfig(t, "127.0.0.1:50017", ""), fixedCoreStatus{state: core.ProcessStopped}, client)
	for _, body := range []string{`{`, `{"tag":"node","runtimeKey":"old","url":"https://example.test"}`, `{"tag":"node"}`, `{"tag":"\n","runtimeKey":"old"}`, `{"tag":"node","runtimeKey":"old"}`} {
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodPost, proxyDelayPath, strings.NewReader(body)))
		if response.Code != 400 && response.Code != 409 {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		response := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(response, httptest.NewRequest(method, proxyDelayPath, nil))
		if response.Code != 405 || response.Header().Get("Allow") != "POST" {
			t.Fatal("incorrect allowed methods")
		}
	}
	if client.calls != 0 {
		t.Fatalf("contacted stopped core %d times", client.calls)
	}
}

func TestProxyDelayRejectsOldRuntimeBeforeAndAfterTesting(t *testing.T) {
	for _, phase := range []string{"before", "validation", "measurement"} {
		t.Run(phase, func(t *testing.T) {
			source := &metricsCoreStatus{status: core.Status{State: core.ProcessRunning, PID: 42, ConfigDirectoryDigest: "old"}}
			var tests atomic.Int32
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/delay") {
					tests.Add(1)
					if phase == "measurement" {
						source.set(core.Status{State: core.ProcessRunning, PID: 43})
					}
					_, _ = w.Write([]byte(`{"delay":100}`))
				} else {
					if phase == "validation" {
						source.set(core.Status{State: core.ProcessRunning, PID: 42, ConfigDirectoryDigest: "new"})
					}
					_, _ = w.Write([]byte(`{"type":"Direct"}`))
				}
			}))
			defer controller.Close()
			service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), ""), source)
			request := delayRequest(t, service, "direct")
			if phase == "before" {
				source.set(core.Status{State: core.ProcessRunning, PID: 43})
			}
			response := httptest.NewRecorder()
			NewHandler(service).ServeHTTP(response, request)
			if response.Code != 409 || !strings.Contains(response.Body.String(), "proxy_runtime_changed") {
				t.Fatalf("accepted stale test: %d %s", response.Code, response.Body.String())
			}
			if phase != "measurement" && tests.Load() != 0 {
				t.Fatal("stale queued test reached core")
			}
		})
	}
}

func TestProxyDelayLimitsConcurrentCoreTestsEvenAfterBrowserCancellation(t *testing.T) {
	started := make(chan struct{}, 5)
	finish := make(chan struct{})
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") {
			started <- struct{}{}
			<-finish
			_, _ = w.Write([]byte(`{"delay":91}`))
		} else {
			_, _ = w.Write([]byte(`{"type":"Direct"}`))
		}
	}))
	defer controller.Close()
	defer close(finish)
	service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), ""), fixedCoreStatus{state: core.ProcessRunning})
	handler := NewHandler(service)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 5)
	for range 5 {
		request := delayRequest(t, service, "direct").WithContext(ctx)
		go func() {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			done <- response.Code
		}()
	}
	for range 5 {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("workers did not start")
		}
	}
	cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, delayRequest(t, service, "direct"))
	if response.Code != 429 || !strings.Contains(response.Body.String(), "proxy_delay_busy") {
		t.Fatalf("limit bypassed: %d %s", response.Code, response.Body.String())
	}
	for range 5 {
		finish <- struct{}{}
	}
	for range 5 {
		if code := <-done; code != 200 {
			t.Fatalf("in-flight test canceled early: %d", code)
		}
	}
}

func TestProxyDelayAllowsResponsesSlowerThanNormalControllerReads(t *testing.T) {
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") {
			time.Sleep(3200 * time.Millisecond)
			_, _ = w.Write([]byte(`{"delay":3200}`))
		} else {
			_, _ = w.Write([]byte(`{"type":"Direct"}`))
		}
	}))
	defer controller.Close()
	service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), ""), fixedCoreStatus{state: core.ProcessRunning})
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, delayRequest(t, service, "direct"))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"delay":3200`) {
		t.Fatalf("premature timeout: %d %s", response.Code, response.Body.String())
	}
}

func TestProxyGroupsExposeOnlyValidMeasuredHistory(t *testing.T) {
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"proxies":{
		"group":{"type":"Selector","now":"first","all":["first","second","third"]},
		"first":{"type":"Trojan","secret":"hidden","history":[{"time":"2026-09-28T01:00:00Z","delay":80},{"time":"2026-09-28T02:00:00Z","delay":90}]},
		"second":{"type":"Direct","history":[{"time":"2026-09-28T02:00:00Z","delay":0}]},
		"third":{"type":"Vless","history":[]}
		}}`))
	}))
	defer controller.Close()
	service := NewService(writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), ""), fixedCoreStatus{state: core.ProcessRunning})
	groups, err := service.ProxyGroups(context.Background())
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	if groups[0].RuntimeKey == "" || groups[0].Members[0].History == nil || groups[0].Members[0].History.Delay != 90 || groups[0].Members[1].History != nil || groups[0].Members[2].History != nil {
		t.Fatalf("history=%+v", groups[0])
	}
	payload, _ := json.Marshal(groups)
	if strings.Contains(string(payload), "hidden") {
		t.Fatal("raw outbound data exposed")
	}
}
