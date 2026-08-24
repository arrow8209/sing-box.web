package subscriptions

import (
	"encoding/json"
	"errors"
	"time"
)

// Subscription is a server-managed collection of complete sing-box outbound
// objects. RawMessage keeps fields that the Web UI does not understand.
type Subscription struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Outbounds []json.RawMessage `json:"outbounds"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// Summary is returned by the collection endpoint so opening the page does not
// load node credentials into the browser until an administrator asks for them.
type Summary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	NodeCount int       `json:"nodeCount"`
	Protocols []string  `json:"protocols"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Name      string            `json:"name"`
	Outbounds []json.RawMessage `json:"outbounds"`
}

var ErrNotFound = errors.New("订阅不存在")
