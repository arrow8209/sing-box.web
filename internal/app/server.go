package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"singbox-web/internal/application"
	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/inbounds"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
	runtimeapi "singbox-web/internal/runtime"
	"singbox-web/internal/settings"
	"singbox-web/internal/storage"
	"singbox-web/internal/subscriptions"
	"singbox-web/internal/templates"
	"singbox-web/internal/updater"
	"singbox-web/internal/webui"
)

// Dependencies provide deterministic dependencies for server construction.
type Dependencies struct {
	Random          io.Reader
	Clock           func() time.Time
	TemplateFactory *templates.Factory
	// Output receives startup information and live core logs. A nil writer
	// keeps embedded and test servers quiet; the CLI supplies its stdout.
	Output io.Writer
	// RuleHTTPTransport is reserved for isolated tests that must replace both
	// Ruleset-Hub and rule-file HTTP requests without accessing the network.
	RuleHTTPTransport http.RoundTripper
	// CoreEnvironment is an explicit allow-list passed to the check process.
	// It is empty in production; integration tests use it for their fake core.
	CoreEnvironment []string
	// ListenAddress is reserved for isolated test servers. Production callers
	// leave it empty and read the Web port from data/settings.json.
	ListenAddress string
	// Listener lets an isolated caller reserve a loopback port before the
	// server starts. Production callers leave it nil.
	Listener net.Listener
	// EnableServerReferenceResolution is an internal rollout/test option.
	EnableServerReferenceResolution bool
}

// Server owns the local HTTP server and the exclusive application lock.
type Server struct {
	layout           paths.Layout
	lock             *storage.InstanceLock
	http             *http.Server
	closeMu          sync.Mutex
	scheduler        *rules.Scheduler
	schedulerCancel  context.CancelFunc
	schedulerDone    chan struct{}
	core             *core.Manager
	coreLog          *core.BoundedLog
	console          *log.Logger
	defaultMixedPort int
	operations       *core.OperationLock
	listener         net.Listener
	closed           bool
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
	if err := subscriptions.ValidateManagedState(layout); err != nil {
		return nil, err
	}
	if err := inbounds.ValidateManagedState(layout); err != nil {
		return nil, err
	}
	if _, err := os.Stat(layout.CoreFile); err == nil {
		if err := storage.ValidateJSON(layout.CoreFile); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	settingsStore, err := settings.OpenStore(layout.SettingsFile)
	if err != nil {
		return nil, err
	}
	coreSettings, err := settingsStore.GetCore()
	if err != nil {
		return nil, err
	}
	serverSettings, err := settingsStore.GetServer()
	if err != nil {
		return nil, err
	}
	factory := deps.TemplateFactory
	if factory == nil {
		factory, err = templates.NewFactoryWithMixedPort(randomSource, serverSettings.DefaultMixedPort)
		if err != nil {
			return nil, err
		}
	}
	if err := profile.RecoverDeletingDrafts(layout); err != nil {
		return nil, err
	}
	subscriptionService, err := subscriptions.Open(layout, clock, randomSource)
	if err != nil {
		return nil, err
	}
	inboundService, err := inbounds.Open(layout, clock, randomSource)
	if err != nil {
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
	downloadTransport := settings.NewDownloadTransport(settingsStore)
	ruleTransport := http.RoundTripper(downloadTransport)
	if deps.RuleHTTPTransport != nil {
		ruleTransport = deps.RuleHTTPTransport
	}
	ruleDownloadPolicy := rules.DownloadPolicy{Transport: ruleTransport}
	ruleService.SetDownloader(rules.NewDownloader(ruleDownloadPolicy))
	catalogFetcher := rules.NewCatalogFetcher(ruleDownloadPolicy)
	ruleCatalog := rules.NewCatalogService(layout.RuleCatalogFile, layout, ruleService, catalogFetcher, clock)
	if err := rules.RecoverRuleTransactions(ruleStore, layout); err != nil {
		return nil, err
	}
	currentDigests, err := ruleService.CurrentDigests(context.Background())
	if err != nil {
		return nil, err
	}
	allDigests := map[string]string{}
	for id, digest := range currentDigests {
		allDigests["rule:"+id] = digest
	}
	inboundDigests, err := inboundService.CurrentDigests(context.Background())
	if err != nil {
		return nil, err
	}
	for key, digest := range inboundDigests {
		allDigests[key] = digest
	}
	subscriptionDigests, err := subscriptionService.CurrentDigests(context.Background())
	if err != nil {
		return nil, err
	}
	for key, digest := range subscriptionDigests {
		allDigests[key] = digest
	}
	if err := profileService.ReconcileAllResourceDigests(context.Background(), allDigests); err != nil {
		return nil, err
	}
	inboundService.SetOnChange(func(ctx context.Context, _ string) {
		if digests, err := inboundService.CurrentDigests(ctx); err == nil {
			_ = profileService.ReconcileTypedResourceDigests(ctx, "inbound", digests)
		}
	})
	subscriptionService.SetOnChange(func(ctx context.Context, _ string) {
		if digests, err := subscriptionService.CurrentDigests(ctx); err == nil {
			_ = profileService.ReconcileTypedResourceDigests(ctx, "subscription", digests)
		}
	})
	if err := configgen.RecoverStagingDirectories(layout.WorkDir); err != nil {
		return nil, fmt.Errorf("恢复档案检查目录: %w", err)
	}
	checker := configgen.NewChecker(layout.WorkDir, configgen.ExecRunner{Environment: append([]string(nil), deps.CoreEnvironment...)}, 30*time.Second, 64<<10)
	operation := core.NewOperationLock()
	var configOptions []configgen.HandlerOption
	if deps.EnableServerReferenceResolution {
		configOptions = append(configOptions, configgen.WithReferenceSources(inboundService, subscriptionService))
	}
	configHandler := configgen.NewHandler(layout, profileService, ruleService, nil, checker, operation, clock, configOptions...)
	coreStore := core.NewStore(layout.CoreFile)
	coreLog, err := core.NewBoundedLog(layout.CoreLog, 1<<20, 16<<10)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !keepLock {
			_ = coreLog.Close()
		}
	}()
	output := deps.Output
	if output == nil {
		output = io.Discard
	}
	console := log.New(output, "", log.LstdFlags)
	coreLog.SetOutput(coreConsoleWriter{console: console})
	coreManager := core.NewManager(layout, coreStore, coreLog, operation)
	coordinator := application.NewCoordinator(profileService, configHandler, nil, coreManager, operation)
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
	} else {
		metas, listErr := profileService.List(context.Background())
		hasAppliedConfig := false
		if listErr == nil {
			for _, meta := range metas {
				if meta.ConfigCurrent && meta.AppliedRevision != nil {
					hasAppliedConfig = true
					break
				}
			}
		}
		switch {
		case listErr != nil:
			coreManager.RecordRecoveryFailure(fmt.Errorf("检查自动启动配置: %w", listErr))
		case hasAppliedConfig && coreSettings.AutoStart:
			if err := coordinator.StartCore(context.Background()); err != nil {
				coreManager.RecordRecoveryFailure(fmt.Errorf("自动启动最近已应用配置: %w", err))
			}
		case state.CurrentProfileID != "" || state.AppliedRevision != nil || state.ExpectedRunning || state.ConfigDirectoryDigest != "":
			if hasAppliedConfig {
				if _, err := coreStore.Update(func(state *core.PersistentState) error {
					state.ExpectedRunning = false
					state.LastStartError = ""
					return nil
				}); err != nil {
					coreManager.RecordRecoveryFailure(fmt.Errorf("记录已关闭的自动启动状态: %w", err))
				}
			} else if err := coreManager.ClearApplication(); err != nil {
				coreManager.RecordRecoveryFailure(fmt.Errorf("清理无配置的自动启动状态: %w", err))
			}
		}
	}
	updateService := updater.NewService(
		layout,
		updater.NewReleaseClient(updater.ReleaseClientPolicy{Transport: downloadTransport}),
		updater.NewFetcher(layout, updater.FetcherPolicy{Transport: downloadTransport}),
		coreManager,
		coreStore,
		checker,
		operation,
	)
	runtimeService := runtimeapi.NewService(layout, coreManager)
	profileService.SetDeleteGuard(coordinator)
	authHandler := auth.NewHandler(authStore)
	capabilityResponse := capabilities.PhaseOne
	capabilityResponse.ServerReferenceResolution = configHandler.ServerReferenceResolution()
	handler := httpapi.NewRouter(httpapi.Dependencies{
		ProfileHandler: profile.NewHandler(profileService, func(ctx context.Context, profileID string, revision uint64) *profile.DraftSaveOutcome {
			behavior, err := settingsStore.GetCore()
			if err != nil {
				return &profile.DraftSaveOutcome{Attempted: true, Message: "草稿已保存，但读取自动重载设置失败"}
			}
			if !behavior.AutoReload {
				return nil
			}
			result, err := coordinator.AutoReload(ctx, application.ApplyRequest{ProfileID: profileID, Revision: revision})
			if !result.Attempted {
				return nil
			}
			outcome := &profile.DraftSaveOutcome{Attempted: true, Applied: result.Applied, Reloaded: result.Reloaded}
			switch {
			case err != nil:
				outcome.Message = "草稿已保存，但自动重载失败，请查看检查结果或手动应用"
			case !result.Applied:
				outcome.Message = result.Validation.Message
				if outcome.Message == "" {
					outcome.Message = "草稿已保存，但配置检查未通过"
				}
			case result.Reloaded:
				outcome.Message = "配置已自动应用并重新加载运行核心"
			default:
				outcome.Message = "配置已自动应用；核心当前保持停止"
			}
			return outcome
		}),
		SubscriptionHandler: subscriptions.NewHandler(subscriptionService),
		InboundHandler:      inbounds.NewHandler(inboundService),
		RuleResourceHandler: rules.NewHandler(ruleService, ruleCatalog),
		ConfigHandler:       configHandler,
		ApplicationHandler:  application.NewHandler(coordinator),
		CoreHandler:         core.NewHandler(coreManager, coordinator),
		UpdaterHandler:      updater.NewHandler(updateService),
		SettingsHandler:     settings.NewHandler(settingsStore),
		RuntimeHandler:      runtimeapi.NewHandler(runtimeService),
		Authenticate:        authHandler.Middleware,
		RequireCSRF:         auth.RequireCSRF,
		AuthHandler:         authHandler,
		CapabilityHandler:   capabilities.NewHandler(capabilityResponse),
		WebUI:               webui.Handler(),
	})
	keepLock = true
	listenAddress := deps.ListenAddress
	if deps.Listener != nil {
		listenAddress = deps.Listener.Addr().String()
	}
	if listenAddress == "" {
		listenAddress = net.JoinHostPort("0.0.0.0", strconv.Itoa(serverSettings.WebPort))
	}
	return &Server{
		layout:           layout,
		lock:             lock,
		http:             &http.Server{Addr: listenAddress, Handler: handler},
		scheduler:        rules.NewScheduler(ruleService, nil),
		schedulerDone:    make(chan struct{}),
		core:             coreManager,
		coreLog:          coreLog,
		console:          console,
		defaultMixedPort: serverSettings.DefaultMixedPort,
		operations:       operation,
		listener:         deps.Listener,
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
	s.printStartup(listener.Addr())
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
// transaction finish, then gracefully stops the child core. The applied
// configuration remains intact, so the next Web startup can run it again.
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
	s.logInfo("正在关闭服务")
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
		coreCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		coreErr = s.core.Stop(coreCtx)
		cancel()
	}
	var logErr error
	if s.coreLog != nil {
		logErr = s.coreLog.Close()
	}
	var lockErr error
	if s.lock != nil {
		lockErr = s.lock.Close()
	}
	err := errors.Join(shutdownErr, operationErr, coreErr, logErr, lockErr)
	if err == nil {
		s.logInfo("服务已停止")
	} else {
		s.logInfo("关闭服务失败：%v", err)
	}
	return err
}

func (s *Server) closeWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+15*time.Second)
	defer cancel()
	return s.Close(ctx)
}
