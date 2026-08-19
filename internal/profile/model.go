package profile

import (
	"encoding/json"
	"time"
)

type ValidationSummary struct {
	Revision  uint64    `json:"revision"`
	Status    string    `json:"status"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checkedAt"`
}

type PublicationState struct {
	Revision    uint64            `json:"revision"`
	Digest      string            `json:"digest"`
	PublishedAt time.Time         `json:"publishedAt"`
	Validation  ValidationSummary `json:"validation"`
}

type Profile struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Slug              string             `json:"slug"`
	DraftRevision     uint64             `json:"draftRevision"`
	PublishedRevision *uint64            `json:"publishedRevision"`
	PublishedDigest   string             `json:"publishedDigest"`
	PublishedAt       *time.Time         `json:"publishedAt"`
	LastValidation    *ValidationSummary `json:"lastValidation"`
}

type EditorBuffer struct {
	Text    string `json:"text"`
	Valid   bool   `json:"valid"`
	Applied bool   `json:"applied"`
	Error   string `json:"error,omitempty"`
}

type DraftContent struct {
	Config        json.RawMessage         `json:"config"`
	EditorBuffers map[string]EditorBuffer `json:"editorBuffers"`
}

type Draft struct {
	ProfileID string       `json:"profileId"`
	Revision  uint64       `json:"revision"`
	Content   DraftContent `json:"content"`
	SavedAt   time.Time    `json:"savedAt"`
}

type CreateInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
