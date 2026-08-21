package auth

import "errors"

// State is the complete, strict on-disk authentication document.
type State struct {
	Version           int    `json:"version"`
	PasswordHash      string `json:"passwordHash"`
	SessionKey        string `json:"sessionKey"`
	SessionGeneration uint64 `json:"sessionGeneration"`
}

const currentVersion = 1

// ErrNotInitialized means the administrator has not created local credentials.
var ErrNotInitialized = errors.New("管理密码尚未初始化")
