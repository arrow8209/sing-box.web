package core

import (
	"fmt"
	"os"
	"sync"

	"singbox-web/internal/storage"
)

// Store atomically reads and updates the durable core state.
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() (PersistentState, error) {
	if s == nil || s.path == "" {
		return PersistentState{}, fmt.Errorf("核心状态文件不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Update serializes a read-modify-write operation and returns the committed
// state. The callback must not retain the supplied pointer.
func (s *Store) Update(update func(*PersistentState) error) (PersistentState, error) {
	if s == nil || s.path == "" {
		return PersistentState{}, fmt.Errorf("核心状态文件不能为空")
	}
	if update == nil {
		return PersistentState{}, fmt.Errorf("核心状态更新不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadLocked()
	if err != nil {
		return PersistentState{}, err
	}
	if err := update(&state); err != nil {
		return PersistentState{}, err
	}
	if err := validatePersistentState(state); err != nil {
		return PersistentState{}, err
	}
	if err := storage.WriteJSON(s.path, state, 0o600); err != nil {
		return PersistentState{}, err
	}
	return clonePersistentState(state), nil
}

// RecordInstalled records only the program that has already been atomically
// installed on disk. Keeping this separate from application state means a
// failed update cannot make durable metadata claim a binary that never ran.
func (s *Store) RecordInstalled(version, digest string) error {
	if version == "" || digest == "" {
		return fmt.Errorf("已安装核心版本或摘要不能为空")
	}
	_, err := s.Update(func(state *PersistentState) error {
		state.InstalledVersion = version
		state.InstalledDigest = digest
		return nil
	})
	return err
}

func (s *Store) loadLocked() (PersistentState, error) {
	state, err := storage.ReadJSON[PersistentState](s.path)
	if os.IsNotExist(err) {
		return PersistentState{Version: persistentStateVersion}, nil
	}
	if err != nil {
		return PersistentState{}, err
	}
	if err := validatePersistentState(state); err != nil {
		return PersistentState{}, &storage.CorruptFileError{Path: s.path, Err: err}
	}
	return clonePersistentState(state), nil
}

func validatePersistentState(state PersistentState) error {
	if state.Version != persistentStateVersion {
		return fmt.Errorf("不支持的核心状态版本: %d", state.Version)
	}
	if state.AppliedRevision != nil && *state.AppliedRevision == 0 {
		return fmt.Errorf("appliedRevision 必须为正整数")
	}
	return nil
}

func clonePersistentState(state PersistentState) PersistentState {
	if state.AppliedRevision != nil {
		revision := *state.AppliedRevision
		state.AppliedRevision = &revision
	}
	if state.LastStartedAt != nil {
		started := *state.LastStartedAt
		state.LastStartedAt = &started
	}
	return state
}
