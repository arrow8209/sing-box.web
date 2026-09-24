package inbounds

import (
	"encoding/json"
	"errors"
	"time"
)

type Mode string

const (
	ModeStructured Mode = "structured"
	ModeCustom     Mode = "custom"
)

// Inbound is a reusable source object. Linked profile entries keep its ID in
// their reference metadata and resolve current Config when generating.
type Inbound struct {
	ID        string          `json:"id"`
	Mode      Mode            `json:"mode"`
	Config    json.RawMessage `json:"config"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// Summary omits the complete object because inbound users can contain
// credentials. The detail endpoint returns Config only on explicit access.
type Summary struct {
	ID         string    `json:"id"`
	Mode       Mode      `json:"mode"`
	Type       string    `json:"type"`
	Tag        string    `json:"tag"`
	Endpoint   string    `json:"endpoint"`
	FieldCount int       `json:"fieldCount"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Input struct {
	Mode   Mode            `json:"mode"`
	Config json.RawMessage `json:"config"`
}

var (
	ErrNotFound     = errors.New("入站配置不存在")
	ErrDuplicateTag = errors.New("入站标签已存在")
)
