package auth

// State is the complete, strict on-disk authentication document.
type State struct {
	Version           int    `json:"version"`
	PasswordHash      string `json:"passwordHash"`
	SessionKey        string `json:"sessionKey"`
	SessionGeneration uint64 `json:"sessionGeneration"`
}

const currentVersion = 1
