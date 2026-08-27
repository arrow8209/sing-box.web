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

// Inbound is a reusable source object. Profiles never retain its ID: importing
// copies Config, applies Routing, and deliberately severs this relation.
type Inbound struct {
	ID        string          `json:"id"`
	Mode      Mode            `json:"mode"`
	Config    json.RawMessage `json:"config"`
	Routing   *RoutingIntent  `json:"routing,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// RoutingIntent stores reusable behavior that belongs to route rules in
// sing-box 1.13 instead of the inbound object itself.
type RoutingIntent struct {
	Sniff        bool   `json:"sniff"`
	SniffTimeout string `json:"sniffTimeout,omitempty"`
}

// Summary omits the complete object because inbound users can contain
// credentials. The detail endpoint returns Config and Routing on explicit access.
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
	Mode    Mode            `json:"mode"`
	Config  json.RawMessage `json:"config"`
	Routing *RoutingIntent  `json:"routing,omitempty"`
}

var (
	ErrNotFound     = errors.New("入站配置不存在")
	ErrDuplicateTag = errors.New("入站标签已存在")
)
