package capabilities

import (
	"context"
	"fmt"
	"sync"
)

type VersionChecker interface {
	Version(context.Context) (string, error)
}

type SingBoxInfo struct {
	Available     bool   `json:"available"`
	Version       string `json:"version"`
	TargetVersion string `json:"targetVersion"`
	Warning       string `json:"warning"`
}

type Snapshot struct {
	Capabilities map[string]bool `json:"capabilities"`
	SingBox      SingBoxInfo     `json:"singBox"`
}

type Service struct {
	mu            sync.RWMutex
	targetVersion string
	checker       VersionChecker
	snapshot      Snapshot
}

func New(targetVersion string, checker VersionChecker) *Service {
	service := &Service{targetVersion: targetVersion, checker: checker}
	service.snapshot = unavailableSnapshot(targetVersion, "sing-box 尚未探测")
	return service
}

func (service *Service) Snapshot(_ context.Context) Snapshot {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return cloneSnapshot(service.snapshot)
}

func (service *Service) Refresh(ctx context.Context) Snapshot {
	version, err := "", error(nil)
	if service.checker == nil {
		err = fmt.Errorf("version checker unavailable")
	} else {
		version, err = service.checker.Version(ctx)
	}

	next := Snapshot{
		Capabilities: declaredCapabilities(),
		SingBox: SingBoxInfo{
			Available:     err == nil,
			Version:       version,
			TargetVersion: service.targetVersion,
		},
	}
	if err != nil {
		next.Capabilities["config.publish"] = false
		next.SingBox.Warning = "sing-box 不可用，配置发布已禁用"
	} else if version != service.targetVersion {
		next.SingBox.Warning = fmt.Sprintf("sing-box 版本不匹配：当前 %s，目标 %s", version, service.targetVersion)
	}

	service.mu.Lock()
	service.snapshot = next
	result := cloneSnapshot(service.snapshot)
	service.mu.Unlock()
	return result
}

func unavailableSnapshot(targetVersion, warning string) Snapshot {
	result := Snapshot{
		Capabilities: declaredCapabilities(),
		SingBox:      SingBoxInfo{TargetVersion: targetVersion, Warning: warning},
	}
	result.Capabilities["config.publish"] = false
	return result
}

func declaredCapabilities() map[string]bool {
	return map[string]bool{
		"config.manage":     true,
		"config.publish":    true,
		"subscription.sync": false,
		"ruleset.sync":      false,
		"core.lifecycle":    false,
		"core.metrics":      false,
	}
}

func cloneSnapshot(source Snapshot) Snapshot {
	result := source
	result.Capabilities = make(map[string]bool, len(source.Capabilities))
	for key, value := range source.Capabilities {
		result.Capabilities[key] = value
	}
	return result
}
