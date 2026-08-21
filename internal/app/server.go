package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	"singbox-web/internal/storage"
	"singbox-web/internal/templates"
	"singbox-web/internal/webui"
)

const defaultListenAddress = "0.0.0.0:50010"

// Dependencies provide deterministic dependencies for server construction.
type Dependencies struct {
	Random          io.Reader
	Clock           func() time.Time
	TemplateFactory *templates.Factory
}

// Server owns the local HTTP server and the exclusive application lock.
type Server struct {
	layout  paths.Layout
	lock    *storage.InstanceLock
	http    *http.Server
	closeMu sync.Mutex
}

// NewServer builds the complete local service. It acquires the lock before
// reading state and releases it on every construction failure.
func NewServer(workDir string, deps Dependencies) (*Server, error) {
	layout, err := paths.Resolve(workDir)
	if err != nil {
		return nil, err
	}
	lock, err := storage.AcquireInstanceLock(layout.AppLock)
	if err != nil {
		return nil, err
	}
	keepLock := false
	defer func() {
		if !keepLock {
			_ = lock.Close()
		}
	}()

	randomSource := deps.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}
	factory := deps.TemplateFactory
	if factory == nil {
		factory, err = templates.NewFactory(randomSource)
		if err != nil {
			return nil, err
		}
	}

	authStore := auth.NewStore(layout.AuthFile, randomSource)
	if _, err := authStore.Load(); err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("%w: %w", auth.ErrNotInitialized, err)
		}
		return nil, fmt.Errorf("请先执行 singbox-web admin set-password: %w", err)
	}
	// Complete every read-only managed JSON gate before recovery or service
	// initialization is allowed to write state.
	if err := profile.ValidateManagedState(layout); err != nil {
		return nil, err
	}
	if _, err := os.Stat(layout.RulesFile); err == nil {
		if err := rules.ValidateManagedState(layout.RulesFile, layout); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, err := os.Stat(layout.CoreFile); err == nil {
		if err := storage.ValidateJSON(layout.CoreFile); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := profile.RecoverDeletingDrafts(layout); err != nil {
		return nil, err
	}
	ruleStore, err := rules.OpenStore(layout.RulesFile, layout, clock)
	if err != nil {
		return nil, err
	}
	bindingSource := NewDefaultBindingSource(factory, ruleStore, randomSource)
	profileService, err := profile.Open(layout, factory, bindingSource, clock, randomSource)
	if err != nil {
		return nil, err
	}
	authHandler := auth.NewHandler(authStore)
	handler := httpapi.NewRouter(httpapi.Dependencies{
		ProfileHandler:    profile.NewHandler(profileService),
		Authenticate:      authHandler.Middleware,
		RequireCSRF:       auth.RequireCSRF,
		AuthHandler:       authHandler,
		CapabilityHandler: capabilities.NewHandler(capabilities.PhaseOne),
		WebUI:             webui.Handler(),
	})
	keepLock = true
	return &Server{
		layout: layout,
		lock:   lock,
		http:   &http.Server{Addr: defaultListenAddress, Handler: handler},
	}, nil
}

// Run listens until the supplied context is cancelled or serving fails. A
// cancelled context gives active requests at most ten seconds to finish.
func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.http == nil {
		return errors.New("服务未初始化")
	}
	listener, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return errors.Join(err, s.closeWithTimeout())
	}
	result := make(chan error, 1)
	go func() {
		result <- s.http.Serve(listener)
	}()
	select {
	case err := <-result:
		closeErr := s.closeWithTimeout()
		if errors.Is(err, http.ErrServerClosed) {
			return closeErr
		}
		return errors.Join(err, closeErr)
	case <-ctx.Done():
		return s.closeWithTimeout()
	}
}

// Close stops HTTP handling with the supplied deadline and always releases the
// instance lock, including when HTTP shutdown times out.
func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	var shutdownErr error
	if s.http != nil {
		shutdownErr = s.http.Shutdown(ctx)
	}
	var lockErr error
	if s.lock != nil {
		lockErr = s.lock.Close()
	}
	return errors.Join(shutdownErr, lockErr)
}

func (s *Server) closeWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.Close(ctx)
}
