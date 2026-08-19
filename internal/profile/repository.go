package profile

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/atomicfile"
)

const indexVersion = 1

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ErrIndexDurabilityUncertain means the new index is visible, but its durability
// could not be confirmed. The operation's returned profile values are valid.
var ErrIndexDurabilityUncertain = errors.New("profile index durability is uncertain")

type indexFile struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

type Repository struct {
	mu             sync.RWMutex
	indexPath      string
	draftsPath     string
	now            func() time.Time
	profiles       []Profile
	writeIndexFile func(string, indexFile) error
	stateErr       error
}

type indexCommitStatus uint8

const (
	indexNotReplaced indexCommitStatus = iota
	indexReplaced
	indexUncertain
)

func OpenRepository(dataDir string, now func() time.Time) (*Repository, error) {
	if now == nil {
		return nil, fmt.Errorf("clock is nil")
	}
	draftsPath := filepath.Join(dataDir, "drafts")
	if err := os.MkdirAll(draftsPath, 0o700); err != nil {
		return nil, fmt.Errorf("create drafts directory: %w", err)
	}
	if err := os.Chmod(draftsPath, 0o700); err != nil {
		return nil, fmt.Errorf("set drafts directory permissions: %w", err)
	}

	indexPath := filepath.Join(dataDir, "profiles.json")
	var disk indexFile
	if err := atomicfile.ReadJSON(indexPath, &disk); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read profile index: %w", err)
		}
		disk = indexFile{Version: indexVersion, Profiles: []Profile{}}
		if err := atomicfile.WriteJSON(indexPath, 0o600, disk); err != nil {
			return nil, fmt.Errorf("initialize profile index: %w", err)
		}
	}
	if disk.Version != indexVersion {
		return nil, fmt.Errorf("unsupported profile index version %d", disk.Version)
	}
	if disk.Profiles == nil {
		disk.Profiles = []Profile{}
	}
	if err := validateProfiles(disk.Profiles); err != nil {
		return nil, fmt.Errorf("validate profile index: %w", err)
	}
	if err := removeOrphanDrafts(draftsPath, disk.Profiles); err != nil {
		return nil, err
	}

	return &Repository{
		indexPath:  indexPath,
		draftsPath: draftsPath,
		now:        now,
		profiles:   cloneProfiles(disk.Profiles),
		writeIndexFile: func(path string, disk indexFile) error {
			return atomicfile.WriteJSON(path, 0o600, disk)
		},
	}, nil
}

func (repo *Repository) List(ctx context.Context) ([]Profile, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if repo.stateErr != nil {
		return nil, repo.stateErr
	}
	return cloneProfiles(repo.profiles), nil
}

func (repo *Repository) Get(ctx context.Context, id string) (Profile, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return Profile{}, err
	}
	if repo.stateErr != nil {
		return Profile{}, repo.stateErr
	}
	if err := validateID(id); err != nil {
		return Profile{}, err
	}
	for _, item := range repo.profiles {
		if item.ID == id {
			return cloneProfile(item), nil
		}
	}
	return Profile{}, fmt.Errorf("profile %q not found", id)
}

func (repo *Repository) Insert(ctx context.Context, item Profile, draft Draft) error {
	_, err := repo.insert(ctx, item, draft)
	return err
}

func (repo *Repository) insert(ctx context.Context, item Profile, draft Draft) (Draft, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}
	if repo.stateErr != nil {
		return Draft{}, repo.stateErr
	}
	if err := validateProfile(item); err != nil {
		return Draft{}, err
	}
	if draft.ProfileID != item.ID {
		return Draft{}, fmt.Errorf("draft profile ID %q does not match profile ID %q", draft.ProfileID, item.ID)
	}
	if draft.Revision != item.DraftRevision {
		return Draft{}, fmt.Errorf("draft revision %d does not match profile revision %d", draft.Revision, item.DraftRevision)
	}
	for _, existing := range repo.profiles {
		if existing.ID == item.ID {
			return Draft{}, fmt.Errorf("profile ID %q already exists", item.ID)
		}
		if existing.Slug == item.Slug {
			return Draft{}, fmt.Errorf("profile slug %q already exists", item.Slug)
		}
	}

	draft.SavedAt = repo.now()
	draftPath := repo.draftPath(item.ID)
	if err := atomicfile.WriteJSON(draftPath, 0o600, draft); err != nil {
		return Draft{}, fmt.Errorf("write profile draft: %w", err)
	}
	next := append(cloneProfiles(repo.profiles), cloneProfile(item))
	status, err := repo.commitIndex(next)
	if status == indexReplaced {
		repo.profiles = next
		return cloneDraft(draft), err
	}
	if status == indexUncertain {
		return Draft{}, err
	}
	if err != nil {
		if removeErr := os.Remove(draftPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return Draft{}, fmt.Errorf("write profile index: %w (rollback draft: %v)", err, removeErr)
		}
		return Draft{}, err
	}
	return Draft{}, fmt.Errorf("profile index was not replaced without an error")
}

func (repo *Repository) Rename(ctx context.Context, id, name string) (Profile, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Profile{}, err
	}
	if repo.stateErr != nil {
		return Profile{}, repo.stateErr
	}
	if err := validateID(id); err != nil {
		return Profile{}, err
	}
	trimmed, err := validateName(name)
	if err != nil {
		return Profile{}, err
	}
	next := cloneProfiles(repo.profiles)
	for index := range next {
		if next[index].ID != id {
			continue
		}
		next[index].Name = trimmed
		status, err := repo.commitIndex(next)
		if status == indexReplaced {
			repo.profiles = next
			return cloneProfile(next[index]), err
		}
		return Profile{}, err
	}
	return Profile{}, fmt.Errorf("profile %q not found", id)
}

func (repo *Repository) RemoveMetadata(ctx context.Context, id string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if repo.stateErr != nil {
		return repo.stateErr
	}
	if err := validateID(id); err != nil {
		return err
	}
	index := -1
	for i, item := range repo.profiles {
		if item.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("profile %q not found", id)
	}
	next := cloneProfiles(repo.profiles[:index])
	next = append(next, cloneProfiles(repo.profiles[index+1:])...)
	status, err := repo.commitIndex(next)
	if status != indexReplaced {
		return err
	}
	repo.profiles = next
	if err != nil {
		return err
	}
	if err := os.Remove(repo.draftPath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove profile draft: %w", err)
	}
	return nil
}

func (repo *Repository) DraftPath(id string) string {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if validateID(id) != nil {
		return ""
	}
	return repo.draftPath(id)
}

func (repo *Repository) draftPath(id string) string {
	return filepath.Join(repo.draftsPath, id+".json")
}

func (repo *Repository) commitIndex(profiles []Profile) (indexCommitStatus, error) {
	disk := indexFile{Version: indexVersion, Profiles: profiles}
	writeErr := repo.writeIndexFile(repo.indexPath, disk)
	if writeErr == nil {
		return indexReplaced, nil
	}

	var current indexFile
	if readErr := atomicfile.ReadJSON(repo.indexPath, &current); readErr != nil {
		repo.stateErr = fmt.Errorf("profile index state is uncertain; reopen repository: write failed: %w; confirmation failed: %v", writeErr, readErr)
		return indexUncertain, repo.stateErr
	}
	if current.Version == indexVersion && profilesEqual(current.Profiles, profiles) {
		return indexReplaced, fmt.Errorf("%w: profile index was replaced but persistence confirmation failed: %w", ErrIndexDurabilityUncertain, writeErr)
	}
	if current.Version == indexVersion && profilesEqual(current.Profiles, repo.profiles) {
		return indexNotReplaced, fmt.Errorf("write profile index: %w", writeErr)
	}
	repo.stateErr = fmt.Errorf("profile index state is uncertain; reopen repository: write failed: %w", writeErr)
	return indexUncertain, repo.stateErr
}

func profilesEqual(left, right []Profile) bool {
	leftJSON, leftErr := json.Marshal(indexFile{Version: indexVersion, Profiles: nonNilProfiles(left)})
	rightJSON, rightErr := json.Marshal(indexFile{Version: indexVersion, Profiles: nonNilProfiles(right)})
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func nonNilProfiles(profiles []Profile) []Profile {
	if profiles == nil {
		return []Profile{}
	}
	return profiles
}

func validateProfiles(profiles []Profile) error {
	ids := make(map[string]struct{}, len(profiles))
	slugs := make(map[string]struct{}, len(profiles))
	for _, item := range profiles {
		if err := validateProfile(item); err != nil {
			return err
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("duplicate profile ID %q", item.ID)
		}
		if _, exists := slugs[item.Slug]; exists {
			return fmt.Errorf("duplicate profile slug %q", item.Slug)
		}
		ids[item.ID] = struct{}{}
		slugs[item.Slug] = struct{}{}
	}
	return nil
}

func validateProfile(item Profile) error {
	if err := validateID(item.ID); err != nil {
		return err
	}
	if _, err := validateName(item.Name); err != nil {
		return err
	}
	if !slugPattern.MatchString(item.Slug) {
		return fmt.Errorf("invalid profile slug %q", item.Slug)
	}
	return nil
}

func validateID(id string) error {
	if len(id) != 32 {
		return fmt.Errorf("invalid profile ID %q", id)
	}
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 || strings.ToLower(id) != id {
		return fmt.Errorf("invalid profile ID %q", id)
	}
	return nil
}

func validateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if !utf8.ValidString(trimmed) {
		return "", fmt.Errorf("profile name is not valid UTF-8")
	}
	count := utf8.RuneCountInString(trimmed)
	if count < 1 || count > 80 {
		return "", fmt.Errorf("profile name must contain 1 to 80 Unicode code points")
	}
	return trimmed, nil
}

func cloneProfiles(profiles []Profile) []Profile {
	result := make([]Profile, len(profiles))
	for index, item := range profiles {
		result[index] = cloneProfile(item)
	}
	return result
}

func cloneProfile(item Profile) Profile {
	if item.PublishedRevision != nil {
		value := *item.PublishedRevision
		item.PublishedRevision = &value
	}
	if item.PublishedAt != nil {
		value := *item.PublishedAt
		item.PublishedAt = &value
	}
	if item.LastValidation != nil {
		value := *item.LastValidation
		item.LastValidation = &value
	}
	return item
}

func removeOrphanDrafts(draftsPath string, profiles []Profile) error {
	wanted := make(map[string]struct{}, len(profiles))
	for _, item := range profiles {
		wanted[item.ID] = struct{}{}
	}
	entries, err := os.ReadDir(draftsPath)
	if err != nil {
		return fmt.Errorf("read drafts directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if validateID(id) != nil {
			continue
		}
		if _, exists := wanted[id]; exists {
			continue
		}
		if err := os.Remove(filepath.Join(draftsPath, entry.Name())); err != nil {
			return fmt.Errorf("remove orphan draft %q: %w", id, err)
		}
	}
	return nil
}
