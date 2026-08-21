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

	"singbox-web/internal/application"
	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	"singbox-web/internal/storage"
	"singbox-web/internal/templates"
	"singbox-web/internal/updater"
	"singbox-web/internal/webui"
)

const defaultListenAddress = "0.0.0.0:50010"

// Dependencies provide deterministic dependencies for server construction.
type Dependencies struct {
	Random          io.Reader
	Clock           func() time.Time
	TemplateFactory *templates.Factory
	// CoreEnvironment is an explicit allow-list passed to the check process.
	// It is empty in production; integration tests use it for their fake core.
	CoreEnvironment []string
	// ListenAddress is reserved for isolated test servers. Production callers
	// leave it empty and always use the documented default listener.
	ListenAddress string
	// Listener lets an isolated caller reserve a loopback port before the
	// server starts. Production callers leave it nil.
	Listener net.Listener
}

// Server owns the local HTTP server and the exclusive application lock.
type Server struct {
	layout          paths.Layout
	lock            *storage.InstanceLock
	http            *http.Server
	closeMu         sync.Mutex
	scheduler       *rules.Scheduler
	schedulerCancel context.CancelFunc
	schedulerDone   chan struct{}
	core            *core.Manager
	operations      *core.OperationLock
	listener        net.Listener
	closed          bool
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
	ruleService, err := rules.NewService(ruleStore, layout, profileService, clock, randomSource)
	if err != nil {
		return nil, err
	}
	if err := rules.RecoverRuleTransactions(ruleStore, layout); err != nil {
		return nil, err
	}
	currentDigests, err := ruleService.CurrentDigests(context.Background())
	if err != nil {
		return nil, err
	}
	if err := profileService.ReconcileResourceDigests(context.Background(), currentDigests); err != nil {
		return nil, err
	}
	checker := configgen.NewChecker(layout.WorkDir, configgen.ExecRunner{Environment: append([]string(nil), deps.CoreEnvironment...)}, 30*time.Second, 64<<10)
	operation := core.NewOperationLock()
	configHandler := configgen.NewHandler(layout, profileService, ruleService, nil, checker, operation, clock, randomSource)
	coreStore := core.NewStore(layout.CoreFile)
	coreLog, err := core.NewBoundedLog(layout.CoreLog, 1<<20, 16<<10)
	if err != nil {
		return nil, err
	}
	coreManager := core.NewManager(layout, coreStore, coreLog, operation)
	// Configuration and program recovery happen only after every managed JSON
	// file passed its read-only validation gate and while app.lock is held.
	state, recoveryErr := coreStore.Load()
	if recoveryErr == nil {
		recoveryErr = application.RecoverConfigTransactions(layout, state.ConfigDirectoryDigest)
	}
	if recoveryErr == nil {
		state, recoveryErr = core.RecoverInstallation(layout, coreStore)
	}
	if recoveryErr != nil {
		// A recovery problem must not lock administrators out of the UI. Core
		// actions stay refused by the start_failed state until repaired.
		coreManager.RecordRecoveryFailure(recoveryErr)
	} else if state.ExpectedRunning {
		metas, listErr := profileService.List(context.Background())
		valid := listErr == nil && state.CurrentProfileID != "" && state.AppliedRevision != nil
		if valid {
			valid = false
			for _, meta := range metas {
				if meta.ID == state.CurrentProfileID && meta.AppliedRevision != nil && *meta.AppliedRevision == *state.AppliedRevision {
					valid = true
					break
				}
			}
		}
		if !valid {
			coreManager.RecordRecoveryFailure(fmt.Errorf("无法恢复核心：当前已应用档案不存在或修订不匹配"))
		} else if err := coreManager.Start(context.Background(), core.StartRequest{ProfileID: state.CurrentProfileID, DraftRevision: *state.AppliedRevision}); err != nil {
			coreManager.RecordRecoveryFailure(err)
		}
	}
	updateService := updater.NewService(layout, updater.NewReleaseClient(), updater.NewFetcher(layout), coreManager, coreStore, checker, operation)
	coordinator := application.NewCoordinator(profileService, ruleService, nil, checker, coreManager, operation)
	profileService.SetDeleteGuard(coordinator)
	authHandler := auth.NewHandler(authStore)
	handler := httpapi.NewRouter(httpapi.Dependencies{
		ProfileHandler:      profile.NewHandler(profileService),
		RuleResourceHandler: rules.NewHandler(ruleService),
		ConfigHandler:       configHandler,
		ApplicationHandler:  application.NewHandler(coordinator),
		CoreHandler:         core.NewHandler(coreManager),
		UpdaterHandler:      updater.NewHandler(updateService),
		Authenticate:        authHandler.Middleware,
		RequireCSRF:         auth.RequireCSRF,
		AuthHandler:         authHandler,
		CapabilityHandler:   capabilities.NewHandler(capabilities.PhaseOne),
		WebUI:               webui.Handler(),
	})
	keepLock = true
	listenAddress := deps.ListenAddress
	if deps.Listener != nil {
		listenAddress = deps.Listener.Addr().String()
	}
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	return &Server{
		layout:        layout,
		lock:          lock,
		http:          &http.Server{Addr: listenAddress, Handler: handler},
		scheduler:     rules.NewScheduler(ruleService, nil),
		schedulerDone: make(chan struct{}),
		core:          coreManager,
		operations:    operation,
		listener:      deps.Listener,
	}, nil
}

// Run listens until the supplied context is cancelled or serving fails. A
// cancelled context gives active requests at most ten seconds to finish.
func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.http == nil {
		return errors.New("服务未初始化")
	}
	if s.scheduler != nil {
		schedulerCtx, cancel := context.WithCancel(ctx)
		s.closeMu.Lock()
		s.schedulerCancel = cancel
		s.closeMu.Unlock()
		go func() { defer close(s.schedulerDone); _ = s.scheduler.Run(schedulerCtx) }()
	}
	listener := s.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", s.http.Addr)
		if err != nil {
			return errors.Join(err, s.closeWithTimeout())
		}
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

// Close stops accepting HTTP requests, cancels scheduling, lets one existing
// transaction finish, then gracefully stops the child core. ExpectedRunning is
// intentionally preserved so a normal Web restart can restore it.
func (s *Server) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var shutdownErr error
	if s.http != nil {
		shutdownErr = s.http.Shutdown(ctx)
	}
	if s.schedulerCancel != nil {
		s.schedulerCancel()
		s.schedulerCancel = nil
		if s.schedulerDone != nil {
			select {
			case <-s.schedulerDone:
			case <-ctx.Done():
			}
		}
	}
	var operationErr, coreErr error
	if s.operations != nil {
		operationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		operationErr = s.operations.WaitIdle(operationCtx)
		cancel()
	}
	if s.core != nil {
		coreCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		coreErr = s.core.Stop(coreCtx)
		cancel()
	}
	var lockErr error
	if s.lock != nil {
		lockErr = s.lock.Close()
	}
	return errors.Join(shutdownErr, operationErr, coreErr, lockErr)
}

func (s *Server) closeWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+10*time.Second)
	defer cancel()
	return s.Close(ctx)
}
