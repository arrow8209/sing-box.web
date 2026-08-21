package rules

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"slices"
)

var (
	ErrResourceNotDownloaded = errors.New("rule_resource_not_downloaded")
	ErrResourceFileDrift     = errors.New("rule_resource_file_drift")
)

// Snapshot is an immutable view of the enabled local rule files. Its release
// callback keeps all listed resources locked until configuration generation is
// complete.
type Snapshot struct {
	Resources map[string]Resource
	Digests   map[string]string
}

func (s *Service) SnapshotForBindings(ctx context.Context, bindings []RuleBinding) (Snapshot, func(), error) {
	ids := make([]string, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.Enabled {
			if _, exists := seen[binding.ResourceID]; !exists {
				seen[binding.ResourceID] = struct{}{}
				ids = append(ids, binding.ResourceID)
			}
		}
	}
	slices.Sort(ids)
	releases := make([]func(), 0, len(ids))
	releaseAll := func() {
		for index := len(releases) - 1; index >= 0; index-- {
			releases[index]()
		}
		releases = nil
	}
	for _, id := range ids {
		release, err := s.locks.Lock(ctx, id)
		if err != nil {
			releaseAll()
			return Snapshot{}, func() {}, err
		}
		releases = append(releases, release)
	}
	snapshot, err := s.snapshotLocked(ids)
	if err != nil {
		releaseAll()
		return Snapshot{}, func() {}, err
	}
	return snapshot, releaseAll, nil
}

func (s *Service) snapshotLocked(ids []string) (Snapshot, error) {
	s.mu.Lock()
	resources, err := s.store.list()
	s.mu.Unlock()
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Resources: make(map[string]Resource, len(ids)), Digests: make(map[string]string, len(ids))}
	for _, id := range ids {
		index := indexOf(resources, id)
		if index < 0 {
			return Snapshot{}, ErrNotFound
		}
		resource := resources[index]
		if resource.Disabled || resource.LastSuccessAt == nil || resource.LastDigest == "" {
			return Snapshot{}, ErrResourceNotDownloaded
		}
		if digest := resourceFileDigest(resource); digest == "" || digest != resource.LastDigest {
			if digest == "" {
				return Snapshot{}, ErrResourceNotDownloaded
			}
			return Snapshot{}, ErrResourceFileDrift
		}
		snapshot.Resources[id] = resource
		snapshot.Digests[id] = resource.LastDigest
	}
	return snapshot, nil
}

func resourceFileDigest(resource Resource) string {
	info, err := os.Lstat(resource.LocalPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	file, err := os.Open(resource.LocalPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// CurrentDigests validates all successful local resources for startup
// reconciliation. It intentionally does not fail for a broken resource: the
// missing digest tells profiles to require a fresh apply.
func (s *Service) CurrentDigests(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	resources, err := s.store.list()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.resourceDigests(resources), nil
}
