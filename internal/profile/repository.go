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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/atomicfile"
)

const indexVersion = 1

var (
	slugPattern                = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	deletionPublicationPattern = regexp.MustCompile(`^([1-9][0-9]*)-([0-9a-f]{12})$`)
)

// ErrIndexDurabilityUncertain means the new index is visible, but its durability
// could not be confirmed. The operation's returned profile values are valid.
var ErrIndexDurabilityUncertain = errors.New("profile index durability is uncertain")

type indexFile struct {
	Version          int                       `json:"version"`
	Profiles         []Profile                 `json:"profiles"`
	PendingDeletions map[string]DeletionIntent `json:"pendingDeletions,omitempty"`
}

type DeletionIntent struct {
	ProfileID string `json:"profileId"`
	Target    string `json:"target"`
	Digest    string `json:"digest"`
	TxID      string `json:"txId"`
	Phase     string `json:"phase"`
}

const (
	DeletionPhasePrepared              = "prepared"
	DeletionPhaseDiskRemovalAuthorized = "disk-removal-authorized"
)

type Repository struct {
	mu               sync.RWMutex
	indexPath        string
	draftsPath       string
	now              func() time.Time
	profiles         []Profile
	pendingDeletions map[string]DeletionIntent
	draftCorrupt     map[string]bool
	writeIndexFile   func(string, indexFile) error
	stateErr         error
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
	if err := validateDeletionIntents(disk.PendingDeletions, disk.Profiles); err != nil {
		return nil, fmt.Errorf("validate pending deletions: %w", err)
	}
	if err := removeOrphanDrafts(draftsPath, disk.Profiles); err != nil {
		return nil, err
	}

	profiles := cloneProfiles(disk.Profiles)
	draftCorrupt := make(map[string]bool)
	indexNeedsRepair := false
	for index := range profiles {
		item := profiles[index]
		var draft Draft
		if err := atomicfile.ReadJSON(filepath.Join(draftsPath, item.ID+".json"), &draft); err != nil {
			draftCorrupt[item.ID] = true
			continue
		}
		if draft.ProfileID != item.ID || draft.Revision == 0 || item.DraftRevision == 0 {
			draftCorrupt[item.ID] = true
			continue
		}
		switch {
		case draft.Revision == item.DraftRevision:
		case draft.Revision == item.DraftRevision+1:
			profiles[index].DraftRevision = draft.Revision
			indexNeedsRepair = true
		default:
			draftCorrupt[item.ID] = true
		}
	}
	if indexNeedsRepair {
		if err := atomicfile.WriteJSON(indexPath, 0o600, indexFile{Version: indexVersion, Profiles: profiles, PendingDeletions: disk.PendingDeletions}); err != nil {
			return nil, fmt.Errorf("repair profile draft revision: %w", err)
		}
	}

	return &Repository{
		indexPath:        indexPath,
		draftsPath:       draftsPath,
		now:              now,
		profiles:         profiles,
		pendingDeletions: cloneDeletionIntents(disk.PendingDeletions),
		draftCorrupt:     draftCorrupt,
		writeIndexFile: func(path string, disk indexFile) error {
			return atomicfile.WriteJSON(path, 0o600, disk)
		},
	}, nil
}

func (repo *Repository) getDraft(ctx context.Context, id string) (Draft, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}
	if repo.stateErr != nil {
		return Draft{}, repo.stateErr
	}
	if err := validateID(id); err != nil {
		return Draft{}, err
	}
	profile, exists := repo.profileByID(id)
	if !exists {
		return Draft{}, fmt.Errorf("profile %q not found", id)
	}
	if repo.draftCorrupt[id] {
		return Draft{}, ErrDraftCorrupt
	}
	var draft Draft
	if err := atomicfile.ReadJSON(repo.draftPath(id), &draft); err != nil || draft.ProfileID != id || draft.Revision == 0 || draft.Revision != profile.DraftRevision {
		repo.draftCorrupt[id] = true
		return Draft{}, ErrDraftCorrupt
	}
	return cloneDraft(draft), nil
}

func (repo *Repository) saveDraft(ctx context.Context, id string, expected uint64, content DraftContent) (Draft, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Draft{}, err
	}
	if repo.stateErr != nil {
		return Draft{}, repo.stateErr
	}
	if err := validateID(id); err != nil {
		return Draft{}, err
	}
	profileIndex := -1
	for index := range repo.profiles {
		if repo.profiles[index].ID == id {
			profileIndex = index
			break
		}
	}
	if profileIndex < 0 {
		return Draft{}, fmt.Errorf("profile %q not found", id)
	}
	if repo.draftCorrupt[id] {
		return Draft{}, ErrDraftCorrupt
	}

	var previous Draft
	if err := atomicfile.ReadJSON(repo.draftPath(id), &previous); err != nil || previous.ProfileID != id || previous.Revision == 0 || previous.Revision != repo.profiles[profileIndex].DraftRevision {
		repo.draftCorrupt[id] = true
		return Draft{}, ErrDraftCorrupt
	}
	current := repo.profiles[profileIndex].DraftRevision
	if expected != previous.Revision || expected != current {
		return Draft{}, &RevisionConflict{Current: current}
	}
	normalized, err := normalizeDraftContent(content)
	if err != nil {
		return Draft{}, err
	}

	draft := Draft{ProfileID: id, Revision: expected + 1, Content: normalized, SavedAt: repo.now()}
	if draft.Revision == 0 {
		return Draft{}, fmt.Errorf("draft revision overflow")
	}
	if err := atomicfile.WriteJSON(repo.draftPath(id), 0o600, draft); err != nil {
		return Draft{}, fmt.Errorf("write profile draft: %w", err)
	}
	next := cloneProfiles(repo.profiles)
	next[profileIndex].DraftRevision = draft.Revision
	status, err := repo.commitIndex(next)
	switch status {
	case indexReplaced:
		repo.profiles = next
		return cloneDraft(draft), err
	case indexNotReplaced:
		if restoreErr := atomicfile.WriteJSON(repo.draftPath(id), 0o600, previous); restoreErr != nil {
			repo.stateErr = fmt.Errorf("profile draft rollback failed; reopen repository: index error: %v; rollback error: %w", err, restoreErr)
			return Draft{}, repo.stateErr
		}
		return Draft{}, err
	case indexUncertain:
		// Preserve the new draft so reopening can resolve the one-step commit,
		// matching the repository's existing uncertain-durability contract.
		return Draft{}, err
	default:
		return Draft{}, fmt.Errorf("profile index was not replaced without an error")
	}
}

func (repo *Repository) draftStatus(ctx context.Context, id string) DraftStatus {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	_, exists := repo.profileByID(id)
	if ctx.Err() != nil || validateID(id) != nil || !exists || !repo.draftCorrupt[id] {
		return DraftStatus{}
	}
	return DraftStatus{ReadOnly: true, Message: draftCorruptMessage}
}

func (repo *Repository) profileByID(id string) (Profile, bool) {
	for _, item := range repo.profiles {
		if item.ID == id {
			return item, true
		}
	}
	return Profile{}, false
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

func (repo *Repository) RecordValidation(ctx context.Context, id string, expectedDraftRevision uint64, summary ValidationSummary) (Profile, error) {
	if summary.Revision != expectedDraftRevision {
		return Profile{}, fmt.Errorf("validation revision %d does not match expected draft revision %d", summary.Revision, expectedDraftRevision)
	}
	return repo.updateProfile(ctx, id, expectedDraftRevision, func(item *Profile) {
		value := summary
		item.LastValidation = &value
	})
}

func (repo *Repository) RecordPublication(ctx context.Context, id string, expectedDraftRevision uint64, state PublicationState) (Profile, error) {
	if state.Revision != expectedDraftRevision || state.Validation.Revision != expectedDraftRevision {
		return Profile{}, fmt.Errorf("publication revision does not match expected draft revision %d", expectedDraftRevision)
	}
	return repo.updateProfile(ctx, id, expectedDraftRevision, func(item *Profile) {
		revision, publishedAt, validation := state.Revision, state.PublishedAt, state.Validation
		item.PublishedRevision = &revision
		item.PublishedDigest = state.Digest
		item.PublishedAt = &publishedAt
		item.LastValidation = &validation
	})
}

func (repo *Repository) RepairPublication(ctx context.Context, id string, state *PublicationState) (Profile, error) {
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
	next := cloneProfiles(repo.profiles)
	for index := range next {
		if next[index].ID != id {
			continue
		}
		if state == nil {
			next[index].PublishedRevision = nil
			next[index].PublishedDigest = ""
			next[index].PublishedAt = nil
		} else {
			if state.Revision == 0 || state.Validation.Revision != state.Revision {
				return Profile{}, fmt.Errorf("invalid recovered publication state")
			}
			revision, publishedAt, validation := state.Revision, state.PublishedAt, state.Validation
			next[index].PublishedRevision = &revision
			next[index].PublishedDigest = state.Digest
			next[index].PublishedAt = &publishedAt
			next[index].LastValidation = &validation
		}
		status, err := repo.commitIndex(next)
		if status == indexReplaced {
			repo.profiles = next
			return cloneProfile(next[index]), err
		}
		return Profile{}, err
	}
	return Profile{}, fmt.Errorf("profile %q not found", id)
}

func (repo *Repository) updateProfile(ctx context.Context, id string, expectedDraftRevision uint64, update func(*Profile)) (Profile, error) {
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
	next := cloneProfiles(repo.profiles)
	for index := range next {
		if next[index].ID != id {
			continue
		}
		if next[index].DraftRevision != expectedDraftRevision {
			return Profile{}, &RevisionConflict{Current: next[index].DraftRevision}
		}
		update(&next[index])
		status, err := repo.commitIndex(next)
		if status == indexReplaced {
			repo.profiles = next
			return cloneProfile(next[index]), err
		}
		return Profile{}, err
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

func (repo *Repository) BeginDeletion(ctx context.Context, id string, intent DeletionIntent) error {
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
	if intent.ProfileID != id {
		return fmt.Errorf("deletion intent profile ID does not match")
	}
	if intent.Phase != DeletionPhasePrepared {
		return fmt.Errorf("deletion intent must begin in prepared phase")
	}
	item, exists := repo.profileByID(id)
	if !exists {
		return fmt.Errorf("profile %q not found", id)
	}
	if err := validateDeletionIntentForProfile(intent, item); err != nil {
		return err
	}
	if _, exists := repo.pendingDeletions[id]; exists {
		return fmt.Errorf("profile deletion is already pending")
	}
	next := cloneDeletionIntents(repo.pendingDeletions)
	if next == nil {
		next = make(map[string]DeletionIntent)
	}
	next[id] = intent
	status, err := repo.commitState(repo.profiles, next)
	if status == indexReplaced {
		repo.pendingDeletions = next
	}
	return err
}

func (repo *Repository) TransitionDeletionPhase(ctx context.Context, id, txID, from, to string) error {
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
	if from != DeletionPhasePrepared || to != DeletionPhaseDiskRemovalAuthorized {
		return fmt.Errorf("invalid deletion phase transition")
	}
	intent, exists := repo.pendingDeletions[id]
	if !exists {
		return fmt.Errorf("profile deletion is not pending")
	}
	if intent.TxID != txID {
		return fmt.Errorf("deletion transaction does not match")
	}
	if intent.Phase != from {
		return fmt.Errorf("deletion phase does not match")
	}
	next := cloneDeletionIntents(repo.pendingDeletions)
	intent.Phase = to
	next[id] = intent
	status, err := repo.commitState(repo.profiles, next)
	if status == indexReplaced {
		repo.pendingDeletions = next
	}
	return err
}

func (repo *Repository) CancelDeletion(ctx context.Context, id, txID string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if repo.stateErr != nil {
		return repo.stateErr
	}
	intent, exists := repo.pendingDeletions[id]
	if !exists {
		return nil
	}
	if intent.TxID != txID {
		return fmt.Errorf("deletion transaction does not match")
	}
	next := cloneDeletionIntents(repo.pendingDeletions)
	delete(next, id)
	status, err := repo.commitState(repo.profiles, next)
	if status == indexReplaced {
		repo.pendingDeletions = next
	}
	return err
}

func (repo *Repository) PendingDeletions(ctx context.Context) ([]DeletionIntent, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if repo.stateErr != nil {
		return nil, repo.stateErr
	}
	result := make([]DeletionIntent, 0, len(repo.pendingDeletions))
	for _, item := range repo.profiles {
		if intent, exists := repo.pendingDeletions[item.ID]; exists {
			result = append(result, intent)
		}
	}
	return result, nil
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
	nextIntents := cloneDeletionIntents(repo.pendingDeletions)
	delete(nextIntents, id)
	status, err := repo.commitState(next, nextIntents)
	if status != indexReplaced {
		return err
	}
	repo.profiles = next
	repo.pendingDeletions = nextIntents
	delete(repo.draftCorrupt, id)
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
	return repo.commitState(profiles, repo.pendingDeletions)
}

func (repo *Repository) commitState(profiles []Profile, pendingDeletions map[string]DeletionIntent) (indexCommitStatus, error) {
	disk := indexFile{Version: indexVersion, Profiles: profiles, PendingDeletions: nonNilDeletionIntents(pendingDeletions)}
	writeErr := repo.writeIndexFile(repo.indexPath, disk)
	if writeErr == nil {
		return indexReplaced, nil
	}

	var current indexFile
	if readErr := atomicfile.ReadJSON(repo.indexPath, &current); readErr != nil {
		repo.stateErr = fmt.Errorf("profile index state is uncertain; reopen repository: write failed: %w; confirmation failed: %v", writeErr, readErr)
		return indexUncertain, repo.stateErr
	}
	if current.Version == indexVersion && indexFilesEqual(current, disk) {
		return indexReplaced, fmt.Errorf("%w: profile index was replaced but persistence confirmation failed: %w", ErrIndexDurabilityUncertain, writeErr)
	}
	previous := indexFile{Version: indexVersion, Profiles: repo.profiles, PendingDeletions: nonNilDeletionIntents(repo.pendingDeletions)}
	if current.Version == indexVersion && indexFilesEqual(current, previous) {
		return indexNotReplaced, fmt.Errorf("write profile index: %w", writeErr)
	}
	repo.stateErr = fmt.Errorf("profile index state is uncertain; reopen repository: write failed: %w", writeErr)
	return indexUncertain, repo.stateErr
}

func indexFilesEqual(left, right indexFile) bool {
	left.Profiles, right.Profiles = nonNilProfiles(left.Profiles), nonNilProfiles(right.Profiles)
	left.PendingDeletions, right.PendingDeletions = nonNilDeletionIntents(left.PendingDeletions), nonNilDeletionIntents(right.PendingDeletions)
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
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

func cloneDeletionIntents(intents map[string]DeletionIntent) map[string]DeletionIntent {
	if intents == nil {
		return nil
	}
	result := make(map[string]DeletionIntent, len(intents))
	for id, intent := range intents {
		result[id] = intent
	}
	return result
}

func nonNilDeletionIntents(intents map[string]DeletionIntent) map[string]DeletionIntent {
	if intents == nil {
		return map[string]DeletionIntent{}
	}
	return intents
}

func validateDeletionIntents(intents map[string]DeletionIntent, profiles []Profile) error {
	profilesByID := make(map[string]Profile, len(profiles))
	for _, item := range profiles {
		profilesByID[item.ID] = item
	}
	for id, intent := range intents {
		item, exists := profilesByID[id]
		if id != intent.ProfileID || !exists {
			return fmt.Errorf("invalid pending deletion for %q", id)
		}
		if err := validateDeletionIntentForProfile(intent, item); err != nil {
			return err
		}
	}
	return nil
}

func validateDeletionIntent(intent DeletionIntent) error {
	if err := validateID(intent.ProfileID); err != nil {
		return err
	}
	if intent.Target == "" {
		return fmt.Errorf("deletion target is empty")
	}
	if len(intent.Digest) != 64 || strings.ToLower(intent.Digest) != intent.Digest {
		return fmt.Errorf("invalid deletion digest")
	}
	if decoded, err := hex.DecodeString(intent.Digest); err != nil || len(decoded) != 32 {
		return fmt.Errorf("invalid deletion digest")
	}
	if intent.Phase != DeletionPhasePrepared && intent.Phase != DeletionPhaseDiskRemovalAuthorized {
		return fmt.Errorf("invalid deletion phase")
	}
	_, prefix, err := parseDeletionTarget(intent.ProfileID, intent.Target)
	if err != nil || prefix != intent.Digest[:12] {
		return fmt.Errorf("invalid deletion target")
	}
	if len(intent.TxID) != 32 || strings.ToLower(intent.TxID) != intent.TxID {
		return fmt.Errorf("invalid deletion transaction ID")
	}
	if decoded, err := hex.DecodeString(intent.TxID); err != nil || len(decoded) != 16 {
		return fmt.Errorf("invalid deletion transaction ID")
	}
	return nil
}

func validateDeletionIntentForProfile(intent DeletionIntent, item Profile) error {
	if err := validateDeletionIntent(intent); err != nil {
		return err
	}
	revision, _, err := parseDeletionTarget(intent.ProfileID, intent.Target)
	if err != nil || item.ID != intent.ProfileID || item.PublishedRevision == nil || *item.PublishedRevision != revision {
		return fmt.Errorf("deletion target does not match published revision")
	}
	if item.PublishedDigest != intent.Digest {
		return fmt.Errorf("deletion digest does not match published digest")
	}
	return nil
}

func parseDeletionTarget(profileID, target string) (uint64, string, error) {
	if target == "" || strings.Contains(target, `\`) || filepath.ToSlash(target) != target || filepath.IsAbs(target) || filepath.Clean(target) != target {
		return 0, "", fmt.Errorf("invalid deletion target")
	}
	parts := strings.Split(target, "/")
	if len(parts) != 3 || parts[0] != ".generations" || parts[1] != profileID {
		return 0, "", fmt.Errorf("invalid deletion target")
	}
	publication := deletionPublicationPattern.FindStringSubmatch(parts[2])
	if publication == nil {
		return 0, "", fmt.Errorf("invalid deletion target")
	}
	revision, err := strconv.ParseUint(publication[1], 10, 64)
	if err != nil || revision == 0 {
		return 0, "", fmt.Errorf("invalid deletion target")
	}
	return revision, publication[2], nil
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
