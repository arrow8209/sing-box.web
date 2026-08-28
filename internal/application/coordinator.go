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

type ApplyRequest struct {
	ProfileID string
	Revision  uint64
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
	if c == nil || c.profiles == nil || c.configuration == nil || c.generator == nil {
		return result, errors.New("应用协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationApply)
	if err != nil {
		return result, err
	}
	defer release()

	// A failed check may refresh only the profile check directory and validation
	// summary. The formal configuration remains untouched.
	operation.SetStage("check_config")
	validation, err := c.configuration.ValidateInOperation(ctx, request.ProfileID, request.Revision)
	result.Validation, result.Digest = validation, validation.Digest
	if err != nil {
		return result, err
	}
	if !validation.Valid {
		return result, nil
	}

	// Reject a draft saved after the successful check before copying anything.
	draft, _, err := c.profiles.GetDraft(ctx, request.ProfileID)
	if err != nil {
		return result, err
	}
	if draft.Revision != request.Revision {
		return result, profile.RevisionConflict{LatestRevision: draft.Revision}
	}

	// Copy the exact in-memory output that produced the successful check. The
	// stable config.staging-<profileID> directory stays available for review.
	token, err := transactionToken()
	if err != nil {
		return result, err
	}
	layout := c.configuration.Layout()
	staging, err := layout.StagingConfigDir(token)
	if err != nil {
		return result, err
	}
	output := configgen.Output{Files: validation.Files, Digest: validation.Digest, ResourceDigests: validation.ResourceDigests}
	if err := c.generator.WriteDirectory(ctx, output, staging); err != nil {
		return result, fmt.Errorf("copy_config: %w", err)
	}
	defer func() { _ = removeOwnedDirectory(staging) }()

	// Digest immediately before the atomic swap. BeginDirectorySwap verifies
	// this digest again after parking the old directory, so a concurrent host
	// edit is never silently overwritten.
	currentDigest, err := DigestDirectory(layout.ConfigDir)
	if err != nil {
		return result, err
	}

	operation.SetStage("copy_config")
	transaction, err := BeginDirectorySwap(layout, staging, currentDigest)
	if err != nil {
		return result, err
	}
	defer transaction.Cleanup()
	now := c.now().UTC()
	applied := profile.AppliedResult{
		ProfileID: request.ProfileID, Revision: request.Revision, Digest: validation.Digest,
		ResourceDigests: validation.ResourceDigests, AppliedAt: now,
		Validation: profile.ValidationSummary{Valid: true, Stage: validation.Stage, Message: validation.Message, CheckedAt: now},
	}
	if err := c.profiles.MarkApplied(ctx, applied); err != nil {
		operation.SetStage("restore_config")
		return result, errors.Join(err, transaction.Restore())
	}

	transaction.Commit()
	result.Applied = true
	return result, nil
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
