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
	"singbox-web/internal/rules"
)

var (
	ErrCheckFailed = errors.New("apply_check_failed")
	ErrStartFailed = errors.New("apply_start_failed")
)

type ApplyRequest struct {
	ProfileID             string
	Revision              uint64
	ConfirmedConfigDigest string
}
type ApplyResult struct {
	ProfileID string `json:"profileId"`
	Revision  uint64 `json:"revision"`
	Digest    string `json:"digest"`
}

// CheckFailure retains the exact core result and the generated configuration
// locations so an administrator can diagnose a failed apply. The randomly
// named directory is the path sing-box actually checked; ReviewConfigDirectory
// is the stable copy retained after that check returns.
type CheckFailure struct {
	Cause                  error
	Result                 configgen.CheckResult
	WorkDir                string
	BinaryPath             string
	CheckedConfigDirectory string
	ReviewConfigDirectory  string
	ConfigFiles            []string
	ProfileID              string
	Revision               uint64
	ConfigDigest           string
	ReviewError            error
}

func (failure *CheckFailure) Error() string {
	if failure == nil || failure.Cause == nil {
		return ErrCheckFailed.Error()
	}
	return fmt.Sprintf("%s: %v", ErrCheckFailed, failure.Cause)
}

func (failure *CheckFailure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Cause
}

func (failure *CheckFailure) Is(target error) bool {
	if failure == nil {
		return false
	}
	return target == ErrCheckFailed || errors.Is(failure.Cause, target)
}

type Coordinator struct {
	profiles   *profile.Service
	rules      *rules.Service
	generator  *configgen.Generator
	checker    *configgen.Checker
	core       *core.Manager
	operations *core.OperationLock
	now        func() time.Time
}

func NewCoordinator(profiles *profile.Service, resources *rules.Service, generator *configgen.Generator, checker *configgen.Checker, manager *core.Manager, operations *core.OperationLock) *Coordinator {
	if generator == nil {
		generator = configgen.NewGenerator()
	}
	if operations == nil {
		operations = core.NewOperationLock()
	}
	return &Coordinator{profiles: profiles, rules: resources, generator: generator, checker: checker, core: manager, operations: operations, now: time.Now}
}

func (c *Coordinator) Apply(ctx context.Context, request ApplyRequest) (result ApplyResult, err error) {
	if c == nil || c.profiles == nil || c.rules == nil || c.checker == nil || c.core == nil {
		return result, errors.New("应用协调器不可用")
	}
	operation, release, err := c.operations.Enter(ctx, core.OperationApply)
	if err != nil {
		return result, err
	}
	defer release()
	operation.SetStage("snapshot")
	draft, _, err := c.profiles.GetDraft(ctx, request.ProfileID)
	if err != nil {
		return result, err
	}
	if draft.Revision != request.Revision {
		return result, profile.RevisionConflict{LatestRevision: draft.Revision}
	}
	snapshot, releaseRules, err := c.rules.SnapshotForBindings(ctx, draft.Bindings)
	if err != nil {
		return result, err
	}
	defer releaseRules()
	previous := c.core.Status(ctx)
	current, err := DigestDirectory(c.core.Layout().ConfigDir)
	if err != nil {
		return result, err
	}
	if _, err := CheckDrift(previous.ConfigDirectoryDigest, request.ConfirmedConfigDigest, current); err != nil {
		return result, err
	}
	operation.SetStage("stop_core")
	if previous.Running() {
		if err := c.core.Stop(ctx); err != nil {
			return result, fmt.Errorf("stop_core: %w", err)
		}
	}
	return c.applyStopped(ctx, operation, request, draft, snapshot, previous)
}

func (c *Coordinator) applyStopped(ctx context.Context, op *core.Operation, request ApplyRequest, draft profile.Draft, snapshot rules.Snapshot, previous core.Status) (result ApplyResult, err error) {
	restartPrevious := func() error {
		if !previous.Running() || previous.CurrentProfileID == "" {
			return nil
		}
		revision := uint64(0)
		if previous.AppliedRevision != nil {
			revision = *previous.AppliedRevision
		}
		return c.core.Start(context.Background(), core.StartRequest{ProfileID: previous.CurrentProfileID, DraftRevision: revision})
	}
	failBeforeSwap := func(cause error) (ApplyResult, error) {
		if restore := restartPrevious(); restore != nil {
			return result, errors.Join(cause, fmt.Errorf("恢复旧核心失败: %w", restore))
		}
		return result, cause
	}
	op.SetStage("generate_config")
	if semantic := configgen.ValidateSemantic(draft, snapshot); len(semantic) > 0 {
		return failBeforeSwap(fmt.Errorf("%w: 配置语义检查失败", ErrCheckFailed))
	}
	output, err := c.generator.Generate(ctx, draft, snapshot)
	if err != nil {
		return failBeforeSwap(fmt.Errorf("generate_config: %w", err))
	}
	token, err := transactionToken()
	if err != nil {
		return failBeforeSwap(err)
	}
	staging, err := c.core.Layout().StagingConfigDir(token)
	if err != nil {
		return failBeforeSwap(err)
	}
	if err := c.generator.WriteDirectory(ctx, output, staging); err != nil {
		_ = os.RemoveAll(staging)
		return failBeforeSwap(fmt.Errorf("generate_config: %w", err))
	}
	defer os.RemoveAll(staging)
	op.SetStage("check_config")
	check, checkErr := c.checker.Check(ctx, c.core.Layout().CoreBinary, staging)
	if checkErr != nil {
		files := make([]string, 0, len(output.Files))
		for _, file := range output.Files {
			files = append(files, file.Name)
		}
		reviewDirectory, reviewErr := preserveFailedCheckDirectory(c.core.Layout(), staging)
		return failBeforeSwap(&CheckFailure{
			Cause:                  checkErr,
			Result:                 check,
			WorkDir:                c.core.Layout().WorkDir,
			BinaryPath:             c.core.Layout().CoreBinary,
			CheckedConfigDirectory: staging,
			ReviewConfigDirectory:  reviewDirectory,
			ConfigFiles:            files,
			ProfileID:              request.ProfileID,
			Revision:               request.Revision,
			ConfigDigest:           output.Digest,
			ReviewError:            reviewErr,
		})
	}
	op.SetStage("swap_config")
	transaction, err := BeginDirectorySwap(c.core.Layout(), staging, previous.ConfigDirectoryDigest)
	if err != nil {
		return result, err
	}
	defer transaction.Cleanup()
	op.SetStage("start_core")
	if err := c.core.Start(ctx, core.StartRequest{ProfileID: request.ProfileID, DraftRevision: request.Revision}); err != nil {
		op.SetStage("recover")
		_ = c.core.Stop(context.Background())
		restore := transaction.Restore()
		restart := restartPrevious()
		return result, errors.Join(fmt.Errorf("%w: %v", ErrStartFailed, err), restore, restart)
	}
	op.SetStage("commit_state")
	now := c.now().UTC()
	applied := profile.AppliedResult{ProfileID: request.ProfileID, Revision: request.Revision, Digest: output.Digest, ResourceDigests: output.ResourceDigests, AppliedAt: now, Validation: profile.ValidationSummary{Valid: true, Stage: "core_check", Message: "配置检查通过", CheckedAt: now}}
	if err := c.profiles.MarkApplied(ctx, applied); err != nil {
		op.SetStage("recover")
		_ = c.core.Stop(context.Background())
		return result, errors.Join(err, transaction.Restore(), restartPrevious())
	}
	if err := c.core.CommitApplication(request.ProfileID, request.Revision, output.Digest); err != nil {
		op.SetStage("recover")
		_ = c.core.Stop(context.Background())
		return result, errors.Join(err, transaction.Restore(), restartPrevious())
	}
	return ApplyResult{ProfileID: request.ProfileID, Revision: request.Revision, Digest: output.Digest}, nil
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
