package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/profile"
	"singbox-web/internal/storage"
)

type ApplyRequest struct {
	ProfileID string
	Revision  uint64
}

type coreStartTarget struct {
	profileID string
	revision  uint64
	digest    string
}

// StartCore starts the sole managed process from the configuration currently
// installed in config/. The durable running intent is committed only after
// the process survives Manager's startup observation window.
func (c *Coordinator) StartCore(ctx context.Context) error {
	if c == nil || c.profiles == nil || c.core == nil {
		return errors.New("核心启动协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationCoreStart)
	if err != nil {
		return err
	}
	defer release()
	operation.SetStage("resolve_config")
	target, err := c.resolveCoreStartTarget(ctx)
	if err != nil {
		operation.SetError(err.Error())
		return err
	}
	return c.startAndCommitCore(ctx, operation, target)
}

// RestartCore preserves the formal config, stops the currently managed
// process and starts a new process using the latest applied config/ owner.
func (c *Coordinator) RestartCore(ctx context.Context) error {
	if c == nil || c.profiles == nil || c.core == nil {
		return errors.New("核心重启协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationCoreRestart)
	if err != nil {
		return err
	}
	defer release()
	if c.core.Status(ctx).State != core.ProcessRunning {
		operation.SetError(core.ErrNotRunning.Error())
		return core.ErrNotRunning
	}
	operation.SetStage("resolve_config")
	target, err := c.resolveCoreStartTarget(ctx)
	if err != nil {
		operation.SetError(err.Error())
		return err
	}
	operation.SetStage("stop_core")
	if err := c.core.Stop(ctx); err != nil {
		operation.SetError(err.Error())
		return err
	}
	return c.startAndCommitCore(ctx, operation, target)
}

func (c *Coordinator) resolveCoreStartTarget(ctx context.Context) (coreStartTarget, error) {
	metas, err := c.profiles.List(ctx)
	if err != nil {
		return coreStartTarget{}, err
	}
	var current *profile.Meta
	for index := range metas {
		if metas[index].ConfigCurrent {
			current = &metas[index]
			break
		}
	}
	if current == nil || current.AppliedRevision == nil || current.QuarantinedError != "" {
		return coreStartTarget{}, core.ErrNoAppliedConfig
	}
	configDir := c.core.Layout().ConfigDir
	if err := storage.RejectSymlinkPath(configDir); err != nil {
		return coreStartTarget{}, fmt.Errorf("%w: %v", core.ErrNoAppliedConfig, err)
	}
	info, err := os.Stat(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return coreStartTarget{}, core.ErrNoAppliedConfig
		}
		return coreStartTarget{}, err
	}
	if !info.IsDir() {
		return coreStartTarget{}, core.ErrNoAppliedConfig
	}
	digest, err := DigestDirectory(configDir)
	if err != nil {
		return coreStartTarget{}, err
	}
	if digest == "" {
		return coreStartTarget{}, core.ErrNoAppliedConfig
	}
	return coreStartTarget{profileID: current.ID, revision: *current.AppliedRevision, digest: digest}, nil
}

func (c *Coordinator) startAndCommitCore(ctx context.Context, operation *core.Operation, target coreStartTarget) error {
	operation.SetStage("start_core")
	if err := c.core.Start(ctx, core.StartRequest{ProfileID: target.profileID, DraftRevision: target.revision}); err != nil {
		operation.SetError(err.Error())
		return err
	}
	operation.SetStage("commit_state")
	if err := c.core.CommitApplication(target.profileID, target.revision, target.digest); err != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		stopErr := c.core.Stop(stopCtx)
		joined := errors.Join(fmt.Errorf("提交核心启动状态: %w", err), stopErr)
		operation.SetError(joined.Error())
		return joined
	}
	return nil
}

// ApplyResult always includes the result produced by the same checker used by
// the standalone 检查 button. Applied remains false when that check fails, and
// in that case config/ has not been changed.
type ApplyResult struct {
	Applied    bool                     `json:"applied"`
	ProfileID  string                   `json:"profileId"`
	Revision   uint64                   `json:"revision"`
	Digest     string                   `json:"digest"`
	Validation configgen.ValidateResult `json:"validation"`
}

// AutoReloadResult distinguishes a non-current profile (not attempted), a
// failed validation, an applied configuration while the core is stopped, and
// a configuration that was also loaded into a running core.
type AutoReloadResult struct {
	Attempted  bool
	Applied    bool
	Reloaded   bool
	Validation configgen.ValidateResult
}

type Coordinator struct {
	profiles      *profile.Service
	configuration *configgen.Handler
	generator     *configgen.Generator
	core          *core.Manager
	operations    *core.OperationLock
	now           func() time.Time
}

func NewCoordinator(profiles *profile.Service, configuration *configgen.Handler, generator *configgen.Generator, manager *core.Manager, operations *core.OperationLock) *Coordinator {
	if generator == nil {
		generator = configgen.NewGenerator()
	}
	if operations == nil {
		operations = core.NewOperationLock()
	}
	return &Coordinator{profiles: profiles, configuration: configuration, generator: generator, core: manager, operations: operations, now: time.Now}
}

func (c *Coordinator) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	result, _, _, err := c.apply(ctx, request, false)
	return result, err
}

// AutoReload applies only when profileID still owns the formal config. A
// running core is restarted under the same global operation lock; a stopped
// core remains stopped after its formal configuration is updated.
func (c *Coordinator) AutoReload(ctx context.Context, request ApplyRequest) (AutoReloadResult, error) {
	result, attempted, reloaded, err := c.apply(ctx, request, true)
	return AutoReloadResult{Attempted: attempted, Applied: result.Applied, Reloaded: reloaded, Validation: result.Validation}, err
}

func (c *Coordinator) apply(ctx context.Context, request ApplyRequest, autoReload bool) (result ApplyResult, attempted bool, reloaded bool, err error) {
	result.ProfileID, result.Revision = request.ProfileID, request.Revision
	if c == nil || c.profiles == nil || c.configuration == nil || c.generator == nil {
		return result, false, false, errors.New("应用协调器不可用")
	}
	if autoReload && c.core == nil {
		return result, false, false, errors.New("核心重载协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationApply)
	if err != nil {
		return result, false, false, err
	}
	defer release()
	if autoReload {
		current, err := c.isCurrentProfile(ctx, request.ProfileID)
		if err != nil {
			operation.SetError(err.Error())
			return result, false, false, err
		}
		if !current {
			return result, false, false, nil
		}
		attempted = true
	}

	// A failed check may refresh only the profile check directory and validation
	// summary. The formal configuration remains untouched.
	operation.SetStage("check_config")
	validation, err := c.configuration.ValidateInOperation(ctx, request.ProfileID, request.Revision)
	result.Validation, result.Digest = validation, validation.Digest
	if err != nil {
		return result, attempted, false, err
	}
	if !validation.Valid {
		return result, attempted, false, nil
	}

	// Reject a draft saved after the successful check before copying anything.
	draft, _, err := c.profiles.GetDraft(ctx, request.ProfileID)
	if err != nil {
		return result, attempted, false, err
	}
	if draft.Revision != request.Revision {
		return result, attempted, false, profile.RevisionConflict{LatestRevision: draft.Revision}
	}

	// Copy the exact in-memory output that produced the successful check. The
	// stable config.staging-<profileID> directory stays available for review.
	token, err := transactionToken()
	if err != nil {
		return result, attempted, false, err
	}
	layout := c.configuration.Layout()
	staging, err := layout.StagingConfigDir(token)
	if err != nil {
		return result, attempted, false, err
	}
	output := configgen.Output{Files: validation.Files, Digest: validation.Digest, ResourceDigests: validation.ResourceDigests}
	if err := c.generator.WriteDirectory(ctx, output, staging); err != nil {
		return result, attempted, false, fmt.Errorf("copy_config: %w", err)
	}
	defer func() { _ = removeOwnedDirectory(staging) }()

	// Digest immediately before the atomic swap. BeginDirectorySwap verifies
	// this digest again after parking the old directory, so a concurrent host
	// edit is never silently overwritten.
	currentDigest, err := DigestDirectory(layout.ConfigDir)
	if err != nil {
		return result, attempted, false, err
	}

	operation.SetStage("copy_config")
	transaction, err := BeginDirectorySwap(layout, staging, currentDigest)
	if err != nil {
		return result, attempted, false, err
	}
	defer transaction.Cleanup()
	now := c.now().UTC()
	applied := profile.AppliedResult{
		ProfileID: request.ProfileID, Revision: request.Revision, Digest: validation.Digest,
		ResourceDigests: validation.ResourceDigests, AppliedAt: now,
		Validation: profile.ValidationSummary{Valid: true, Stage: validation.Stage, Message: validation.Message, CheckedAt: now, ResourceDigests: validation.ResourceDigests, ConfigDigest: validation.Digest},
	}
	if err := c.profiles.MarkApplied(ctx, applied); err != nil {
		operation.SetStage("restore_config")
		return result, attempted, false, errors.Join(err, transaction.Restore())
	}

	transaction.Commit()
	result.Applied = true
	if !autoReload || !c.core.Status(ctx).Running() {
		return result, attempted, false, nil
	}
	operation.SetStage("stop_core")
	if err := c.core.Stop(ctx); err != nil {
		operation.SetError(err.Error())
		return result, attempted, false, err
	}
	operation.SetStage("resolve_config")
	target, err := c.resolveCoreStartTarget(ctx)
	if err != nil {
		operation.SetError(err.Error())
		return result, attempted, false, err
	}
	if err := c.startAndCommitCore(ctx, operation, target); err != nil {
		return result, attempted, false, err
	}
	return result, attempted, true, nil
}

func (c *Coordinator) isCurrentProfile(ctx context.Context, profileID string) (bool, error) {
	metas, err := c.profiles.List(ctx)
	if err != nil {
		return false, err
	}
	for _, meta := range metas {
		if meta.ConfigCurrent {
			return meta.ID == profileID && meta.AppliedRevision != nil && meta.QuarantinedError == "", nil
		}
	}
	return false, nil
}

func transactionToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// GuardDelete lets profile.Service retain its atomic metadata/draft deletion
// while this layer owns the potentially dangerous core/config transition.
func (c *Coordinator) GuardDelete(ctx context.Context, profileID string, stopCurrent bool) error {
	if c == nil || c.core == nil {
		return profile.ErrCurrentProfileRequiresCoreDecision
	}
	status := c.core.Status(ctx)
	if status.CurrentProfileID != profileID {
		return nil
	}
	if !stopCurrent {
		return profile.ErrCurrentProfileRequiresCoreDecision
	}
	op, release, err := c.operations.Enter(ctx, core.OperationApply)
	if err != nil {
		return err
	}
	defer release()
	op.SetStage("stop_core")
	if err := c.core.Stop(ctx); err != nil {
		return fmt.Errorf("stop_core: %w", err)
	}
	config := c.core.Layout().ConfigDir
	parked := filepath.Join(c.core.Layout().WorkDir, "config.deleting-"+profileID)
	if err := os.Rename(config, parked); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := c.core.ClearApplication(); err != nil {
		_ = os.Rename(parked, config)
		return err
	}
	_ = os.RemoveAll(parked)
	return nil
}
