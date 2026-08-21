package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"singbox-web/internal/core"
)

func TestOperationLockPublishesCurrentStage(t *testing.T) {
	lock := core.NewOperationLock()
	operation, release, err := lock.Enter(context.Background(), core.OperationApply)
	if err != nil {
		t.Fatal(err)
	}
	operation.SetStage("check_config")
	status := lock.Status()
	if status.Kind != "apply" || status.Stage != "check_config" || !status.Active {
		t.Fatalf("status=%+v", status)
	}
	release()
	if lock.Status().Active {
		t.Fatal("operation remained active")
	}
}

func TestOperationLockWaitHonorsContext(t *testing.T) {
	lock := core.NewOperationLock()
	_, release, err := lock.Enter(context.Background(), core.OperationApply)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, _, err = lock.Enter(ctx, core.OperationValidate)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}
