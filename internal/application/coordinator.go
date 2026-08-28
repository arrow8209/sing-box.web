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
)

var ErrStartFailed = errors.New("apply_start_failed")

type ApplyRequest struct {
	ProfileID             string
	Revision              uint64
	ConfirmedConfigDigest string
}

// ApplyResult always includes the result produced by the same checker used by
// the standalone 检查 button. Applied remains false when that check fails, and
// in that case neither the core process nor config/ has been changed.
type ApplyResult struct {
	Applied    bool                     `json:"applied"`
	ProfileID  string                   `json:"profileId"`
	Revision   uint64                   `json:"revision"`
	Digest     string                   `json:"digest"`
	Validation configgen.ValidateResult `json:"validation"`
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

func (c *Coordinator) Apply(ctx context.Context, request ApplyRequest) (result ApplyResult, err error) {
	result.ProfileID, result.Revision = request.ProfileID, request.Revision
	if c == nil || c.profiles == nil || c.configuration == nil || c.generator == nil || c.core == nil {
		return result, errors.New("应用协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationApply)
	if err != nil {
		return result, err
	}
	defer release()

	// A failed check may refresh only the profile check directory and validation
	// summary. The running process and formal configuration remain untouched.
	operation.SetStage("check_config")
	validation, err := c.configuration.ValidateInOperation(ctx, request.ProfileID, request.Revision)
	result.Validation, result.Digest = validation, validation.Digest
	if err != nil {
		return result, err
	}
	if !validation.Valid {
		return result, nil
	}

	// Reject a draft saved after the successful check before stopping anything.
	draft, _, err := c.profiles.GetDraft(ctx, request.ProfileID)
	if err != nil {
		return result, err
	}
	if draft.Revision != request.Revision {
		return result, profile.RevisionConflict{LatestRevision: draft.Revision}
	}

	previous := c.core.Status(ctx)
	currentDigest, err := DigestDirectory(c.core.Layout().ConfigDir)
	if err != nil {
		return result, err
	}
	if _, err := CheckDrift(previous.ConfigDirectoryDigest, request.ConfirmedConfigDigest, currentDigest); err != nil {
		return result, err
	}

	// Copy the exact in-memory output that produced the successful check. The
	// stable config.staging-<profileID> directory stays available for review.
	token, err := transactionToken()
	if err != nil {
		return result, err
	}
	staging, err := c.core.Layout().StagingConfigDir(token)
	if err != nil {
		return result, err
	}
	output := configgen.Output{Files: validation.Files, Digest: validation.Digest, ResourceDigests: validation.ResourceDigests}
	if err := c.generator.WriteDirectory(ctx, output, staging); err != nil {
		return result, fmt.Errorf("copy_config: %w", err)
	}
	defer func() { _ = removeOwnedDirectory(staging) }()

	operation.SetStage("stop_core")
	if previous.Running() {
		if err := c.core.Stop(ctx); err != nil {
			return result, fmt.Errorf("stop_core: %w", err)
		}
	}

	// Close the host-edit window between the initial drift check and Stop.
	latestDigest, err := DigestDirectory(c.core.Layout().ConfigDir)
	if err != nil {
		return result, errors.Join(err, c.restartPrevious(previous))
	}
	if latestDigest != currentDigest {
		return result, errors.Join(fmt.Errorf("%w: 正式配置在停止核心前发生变更", ErrExternalDrift), c.restartPrevious(previous))
	}

	operation.SetStage("swap_config")
	transaction, err := BeginDirectorySwap(c.core.Layout(), staging, currentDigest)
	if err != nil {
		return result, errors.Join(err, c.restartPrevious(previous))
	}
	defer transaction.Cleanup()
	directoryDigest, err := DigestDirectory(c.core.Layout().ConfigDir)
	if err != nil {
		return result, c.recover(previous, transaction, err)
	}

	operation.SetStage("start_core")
	if err := c.core.Start(ctx, core.StartRequest{ProfileID: request.ProfileID, DraftRevision: request.Revision}); err != nil {
		return result, c.recover(previous, transaction, fmt.Errorf("%w: %v", ErrStartFailed, err))
	}

	operation.SetStage("commit_state")
	if err := c.core.CommitApplication(request.ProfileID, request.Revision, directoryDigest); err != nil {
		return result, c.recover(previous, transaction, err)
	}
	now := c.now().UTC()
	applied := profile.AppliedResult{
		ProfileID: request.ProfileID, Revision: request.Revision, Digest: validation.Digest,
		ResourceDigests: validation.ResourceDigests, AppliedAt: now,
		Validation: profile.ValidationSummary{Valid: true, Stage: validation.Stage, Message: validation.Message, CheckedAt: now},
	}
	if err := c.profiles.MarkApplied(ctx, applied); err != nil {
		return result, c.recover(previous, transaction, err)
	}

	transaction.Commit()
	result.Applied = true
	return result, nil
}

func (c *Coordinator) restartPrevious(previous core.Status) error {
	if !previous.Running() {
		return c.core.RestoreApplication(previous)
	}
	revision := uint64(0)
	if previous.AppliedRevision != nil {
		revision = *previous.AppliedRevision
	}
	startErr := c.core.Start(context.Background(), core.StartRequest{ProfileID: previous.CurrentProfileID, DraftRevision: revision})
	stateErr := c.core.RestoreApplication(previous)
	if startErr != nil {
		return errors.Join(fmt.Errorf("恢复旧核心失败: %w", startErr), stateErr)
	}
	return stateErr
}

func (c *Coordinator) recover(previous core.Status, transaction *DirectoryTransaction, cause error) error {
	if stopErr := c.core.Stop(context.Background()); stopErr != nil {
		// Keep both directories intact when the new process cannot be stopped.
		// Startup recovery can resolve them after process ownership is gone.
		return errors.Join(cause, fmt.Errorf("停止待恢复核心失败: %w", stopErr))
	}
	restoreErr := transaction.Restore()
	restartErr := c.restartPrevious(previous)
	return errors.Join(cause, restoreErr, restartErr)
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
