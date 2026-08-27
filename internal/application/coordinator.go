package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"singbox-web/internal/core"
	"singbox-web/internal/profile"
)

// Coordinator currently owns only the core-aware profile deletion guard.
// Configuration application is intentionally not implemented.
type Coordinator struct {
	core       *core.Manager
	operations *core.OperationLock
}

func NewCoordinator(manager *core.Manager, operations *core.OperationLock) *Coordinator {
	if operations == nil {
		operations = core.NewOperationLock()
	}
	return &Coordinator{core: manager, operations: operations}
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
