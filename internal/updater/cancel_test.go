package updater

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"singbox-web/internal/core"
)

type cancellationReleaseLookup struct{}

func (cancellationReleaseLookup) LatestStable(context.Context) (Release, error) {
	return Release{Version: "1.12.3"}, nil
}

type blockingProgressPreparer struct{ started chan struct{} }

func (p *blockingProgressPreparer) Prepare(ctx context.Context, release Release) (PreparedBinary, func(), error) {
	return p.PrepareWithProgress(ctx, release, nil)
}

func (p *blockingProgressPreparer) PrepareWithProgress(ctx context.Context, _ Release, report func(DownloadProgress)) (PreparedBinary, func(), error) {
	if report != nil {
		report(DownloadProgress{DownloadedBytes: 8 << 20, TotalBytes: 32 << 20, BytesPerSecond: 4 << 20})
	}
	close(p.started)
	<-ctx.Done()
	return PreparedBinary{}, nil, ctx.Err()
}

func TestCancelStopsActiveDownload(t *testing.T) {
	preparer := &blockingProgressPreparer{started: make(chan struct{})}
	service := &Service{
		releases: cancellationReleaseLookup{},
		fetcher:  preparer,
		core:     &core.Manager{},
		store:    &core.Store{},
		random:   bytes.NewReader(make([]byte, 16)),
	}
	operationID, err := service.Start(context.Background(), UpdateConfirmation{ConfirmedVersion: "1.12.3"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-preparer.started:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	cancelledID, err := service.Cancel()
	if err != nil || cancelledID != operationID {
		t.Fatalf("cancelledID=%q err=%v", cancelledID, err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := service.Status()
		if status.FinishedAt != nil {
			if status.Stage != "cancelled" || status.Message != "下载已停止" || status.BytesPerSecond != 0 {
				t.Fatalf("status=%+v", status)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("cancel did not finish: %+v", service.Status())
}

func TestCancelRejectsNonDownloadStage(t *testing.T) {
	called := false
	service := &Service{
		active: true,
		status: Status{OperationID: "update-a", Stage: "installing"},
		cancel: func() { called = true },
	}
	if _, err := service.Cancel(); !errors.Is(err, ErrUpdateNotCancellable) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("installing operation was cancelled")
	}
}
