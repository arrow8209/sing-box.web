package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"

	"singbox-web/internal/storage"
)

type Store struct {
	file   string
	random io.Reader
	mu     sync.Mutex
}

func NewStore(file string, randomSource io.Reader) *Store {
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &Store{file: file, random: randomSource}
}

func (s *Store) Load() (State, error) {
	state, err := storage.ReadJSON[State](s.file)
	if err != nil {
		return State{}, err
	}
	if err := validateState(state); err != nil {
		return State{}, &storage.CorruptFileError{Path: s.file, Err: err}
	}
	return state, nil
}

func (s *Store) SetPassword(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, err := Hash(password, s.random)
	if err != nil {
		return err
	}
	state := State{}
	if existing, loadErr := s.Load(); loadErr == nil {
		state = existing
	} else if !os.IsNotExist(loadErr) {
		return loadErr
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(s.random, key); err != nil {
		return err
	}
	state.Version = currentVersion
	state.PasswordHash = hash
	state.SessionKey = base64.RawStdEncoding.EncodeToString(key)
	state.SessionGeneration++
	if err := storage.WriteJSON(s.file, state, 0o600); err != nil {
		return fmt.Errorf("写入认证状态失败: %w", err)
	}
	return nil
}

func validateState(state State) error {
	if state.Version != currentVersion {
		return fmt.Errorf("不支持的认证状态版本")
	}
	if _, _, _, err := parseHash(state.PasswordHash); err != nil {
		return err
	}
	key, err := base64.RawStdEncoding.DecodeString(state.SessionKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("无效的会话签名密钥")
	}
	if state.SessionGeneration == 0 {
		return fmt.Errorf("无效的会话代次")
	}
	return nil
}
