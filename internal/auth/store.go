package auth

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"singbox-web/internal/atomicfile"
)

type Record struct {
	Version      int       `json:"version"`
	PasswordHash string    `json:"passwordHash"`
	SessionKey   string    `json:"sessionKey"`
	SessionEpoch uint64    `json:"sessionEpoch"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type FileStore struct {
	path   string
	now    func() time.Time
	random io.Reader
}

func NewFileStore(path string, now func() time.Time, random io.Reader) *FileStore {
	return &FileStore{path: path, now: now, random: random}
}

func (store *FileStore) SetPassword(password []byte) error {
	if utf8.RuneCount(password) < 12 {
		return ErrWeakPassword
	}
	previous, err := store.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	hash, err := HashPassword(password, store.random)
	if err != nil {
		return err
	}
	sessionKey := make([]byte, 32)
	if _, err := io.ReadFull(store.random, sessionKey); err != nil {
		return err
	}
	record := Record{
		Version:      1,
		PasswordHash: hash,
		SessionKey:   hex.EncodeToString(sessionKey),
		SessionEpoch: previous.SessionEpoch + 1,
		UpdatedAt:    store.now().UTC(),
	}
	clear(sessionKey)
	return atomicfile.WriteJSON(store.path, 0o600, record)
}

func (store *FileStore) Authenticate(password []byte) (Record, bool, error) {
	record, err := store.Load()
	if err != nil {
		return Record{}, false, err
	}
	ok, err := VerifyPassword(password, record.PasswordHash)
	if err != nil {
		return Record{}, false, err
	}
	if !ok {
		return Record{}, false, nil
	}
	return record, true, nil
}

func (store *FileStore) Load() (Record, error) {
	var record Record
	if err := atomicfile.ReadJSON(store.path, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}
