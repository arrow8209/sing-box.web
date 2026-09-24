package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"singbox-web/internal/core"
)

// MetricsSnapshot contains only aggregate counters, not connection metadata.
// RuntimeKey and ObservedAt let clients compute rates without mixing core runs
// or mistaking HTTP polling delays for a fixed one-second sampling interval.
type MetricsSnapshot struct {
	Memory          int64     `json:"memory"`
	UploadTotal     int64     `json:"uploadTotal"`
	DownloadTotal   int64     `json:"downloadTotal"`
	ConnectionCount int       `json:"connectionCount"`
	RuntimeKey      string    `json:"runtimeKey"`
	ObservedAt      time.Time `json:"observedAt"`
}

func runtimeKey(status core.Status) string {
	var startedAt string
	if status.StartedAt != nil {
		startedAt = status.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%d:%s", status.PID, startedAt)
}

func (s *Service) Metrics(ctx context.Context) (MetricsSnapshot, error) {
	if s == nil || s.core == nil {
		return MetricsSnapshot{}, ErrControllerUnavailable
	}
	before := s.core.Status(ctx)
	if before.State != core.ProcessRunning {
		return MetricsSnapshot{}, ErrCoreNotRunning
	}
	payload, err := s.readConnectionsPayload(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	observedAt := time.Now().UTC()
	after := s.core.Status(ctx)
	if after.State != core.ProcessRunning {
		return MetricsSnapshot{}, ErrCoreNotRunning
	}
	if runtimeKey(before) != runtimeKey(after) {
		return MetricsSnapshot{}, fmt.Errorf("%w: core changed during sampling", ErrControllerUnavailable)
	}
	var wire struct {
		Memory        *int64          `json:"memory"`
		UploadTotal   *int64          `json:"uploadTotal"`
		DownloadTotal *int64          `json:"downloadTotal"`
		Connections   json.RawMessage `json:"connections"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil || wire.Memory == nil || wire.UploadTotal == nil || wire.DownloadTotal == nil || len(wire.Connections) == 0 {
		return MetricsSnapshot{}, fmt.Errorf("%w: incomplete metrics response", ErrControllerUnavailable)
	}
	if *wire.Memory < 0 || *wire.UploadTotal < 0 || *wire.DownloadTotal < 0 {
		return MetricsSnapshot{}, fmt.Errorf("%w: invalid metrics counters", ErrControllerUnavailable)
	}
	// sing-box returns null when there are no active connections. A missing or
	// malformed field must not silently become a real zero on the dashboard.
	var connections []json.RawMessage
	if err := json.Unmarshal(wire.Connections, &connections); err != nil {
		return MetricsSnapshot{}, fmt.Errorf("%w: invalid connections response", ErrControllerUnavailable)
	}
	return MetricsSnapshot{
		Memory: *wire.Memory, UploadTotal: *wire.UploadTotal, DownloadTotal: *wire.DownloadTotal,
		ConnectionCount: len(connections), RuntimeKey: runtimeKey(after), ObservedAt: observedAt,
	}, nil
}
