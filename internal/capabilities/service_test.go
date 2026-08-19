package capabilities

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"singbox-web/internal/singbox"
)

type versionCheckerFunc func(context.Context) (string, error)

func (fn versionCheckerFunc) Version(ctx context.Context) (string, error) { return fn(ctx) }

var exactCapabilities = map[string]bool{
	"config.manage":     true,
	"config.publish":    true,
	"subscription.sync": false,
	"ruleset.sync":      false,
	"core.lifecycle":    false,
	"core.metrics":      false,
}

func TestRefreshPublishesExactCapabilities(t *testing.T) {
	service := New("1.12.0", versionCheckerFunc(func(context.Context) (string, error) {
		return "1.12.0", nil
	}))
	got := service.Refresh(context.Background())
	if !reflect.DeepEqual(got.Capabilities, exactCapabilities) {
		t.Fatalf("capabilities = %#v", got.Capabilities)
	}
	if got.SingBox != (SingBoxInfo{Available: true, Version: "1.12.0", TargetVersion: "1.12.0"}) {
		t.Fatalf("singBox = %#v", got.SingBox)
	}
}

func TestRefreshUnavailableDisablesOnlyPublish(t *testing.T) {
	service := New("1.12.0", versionCheckerFunc(func(context.Context) (string, error) {
		return "", singbox.ErrUnavailable
	}))
	got := service.Refresh(context.Background())
	want := cloneCapabilities(exactCapabilities)
	want["config.publish"] = false
	if !reflect.DeepEqual(got.Capabilities, want) || got.SingBox.Available || got.SingBox.Warning == "" {
		t.Fatalf("snapshot = %#v", got)
	}
}

func TestRefreshVersionMismatchWarnsButKeepsPublish(t *testing.T) {
	service := New("1.12.0", versionCheckerFunc(func(context.Context) (string, error) {
		return "1.11.9", nil
	}))
	got := service.Refresh(context.Background())
	if !got.Capabilities["config.publish"] || !got.SingBox.Available || got.SingBox.Version != "1.11.9" || got.SingBox.TargetVersion != "1.12.0" || got.SingBox.Warning == "" {
		t.Fatalf("snapshot = %#v", got)
	}
}

func TestSnapshotUsesCacheAndReturnsIndependentMap(t *testing.T) {
	calls := 0
	service := New("1.12.0", versionCheckerFunc(func(context.Context) (string, error) {
		calls++
		return "1.12.0", nil
	}))
	service.Refresh(context.Background())
	first := service.Snapshot(context.Background())
	first.Capabilities["config.manage"] = false
	second := service.Snapshot(context.Background())
	if calls != 1 || !second.Capabilities["config.manage"] {
		t.Fatalf("calls=%d snapshot=%#v", calls, second)
	}
}

func TestRefreshOtherProbeErrorIsUnavailable(t *testing.T) {
	service := New("1.12.0", versionCheckerFunc(func(context.Context) (string, error) {
		return "", errors.New("private probe detail")
	}))
	got := service.Refresh(context.Background())
	if got.SingBox.Available || got.Capabilities["config.publish"] || got.SingBox.Warning == "" {
		t.Fatalf("snapshot = %#v", got)
	}
}

func cloneCapabilities(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
