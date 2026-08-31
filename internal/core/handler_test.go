package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"singbox-web/internal/paths"
)

func TestHandlerReturnsCoreStatusAndRejectsUnsupportedMethods(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := &fakeLifecycle{}
	handler := NewHandler(NewManagerWithFactory(layout, nil, nil, nil, &fakeFactory{}), lifecycle)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/core", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "not_installed") {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/core", nil))
	if response.Code != http.StatusOK || lifecycle.starts != 1 {
		t.Fatalf("response=%d starts=%d", response.Code, lifecycle.starts)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/core/restart", nil))
	if response.Code != http.StatusOK || lifecycle.restarts != 1 {
		t.Fatalf("response=%d restarts=%d", response.Code, lifecycle.restarts)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/core", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("response=%d", response.Code)
	}
}

func TestHandlerReportsMissingAppliedConfiguration(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewManagerWithFactory(layout, nil, nil, nil, &fakeFactory{}), &fakeLifecycle{startErr: ErrNoAppliedConfig})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/core", nil))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"core_config_not_applied"`) {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}

func TestHandlerDeleteWaitsForManagedProcessExit(t *testing.T) {
	manager, factory, clock := newTestManager(t)
	started := make(chan error, 1)
	go func() { started <- manager.Start(t.Context(), StartRequest{}) }()
	await(t, func() bool { return factory.LastProcess() != nil })
	await(t, clock.HasWaiter)
	clock.Advance(2 * time.Second)
	if err := <-started; err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		NewHandler(manager).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/core", nil))
		close(done)
	}()
	process := factory.LastProcess()
	await(t, func() bool { return process.HasSignal(syscall.SIGTERM) })
	select {
	case <-done:
		t.Fatal("DELETE returned before the managed process exited")
	default:
	}
	process.Exit(ProcessResult{})
	<-done
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"stopped"`) || !strings.Contains(response.Body.String(), `"pid":0`) {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}

type fakeLifecycle struct {
	starts     int
	restarts   int
	startErr   error
	restartErr error
}

func (l *fakeLifecycle) StartCore(context.Context) error {
	l.starts++
	return l.startErr
}

func (l *fakeLifecycle) RestartCore(context.Context) error {
	l.restarts++
	return l.restartErr
}
