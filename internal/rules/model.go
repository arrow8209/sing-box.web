package rules

import (
	"context"
	"fmt"
	"time"
)

type Format string

const (
	FormatSource Format = "source"
	FormatBinary Format = "binary"
)

type ResourceTemplate struct {
	TemplateKey    string `json:"templateKey"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         Format `json:"format"`
	UpdateInterval string `json:"updateInterval"`
}

type Resource struct {
	ID             string     `json:"id"`
	TemplateKey    string     `json:"templateKey,omitempty"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Format         Format     `json:"format"`
	UpdateInterval string     `json:"updateInterval"`
	Disabled       bool       `json:"disabled"`
	LocalPath      string     `json:"localPath"`
	LastAttemptAt  *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt,omitempty"`
	LastDigest     string     `json:"lastDigest,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	NextUpdateAt   *time.Time `json:"nextUpdateAt,omitempty"`
}

type CreateInput struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         Format `json:"format"`
	UpdateInterval string `json:"updateInterval"`
	Disabled       bool   `json:"disabled"`
}

type PatchInput struct {
	Name           *string `json:"name"`
	URL            *string `json:"url"`
	Format         *Format `json:"format"`
	UpdateInterval *string `json:"updateInterval"`
	Disabled       *bool   `json:"disabled"`
}

// RuleBinding is shared with the profile package. Keeping the concrete type in
// rules lets the rules service create a typed snapshot without an import cycle.
type RuleBinding struct {
	ResourceID string `json:"resourceId"`
	Tag        string `json:"tag"`
	Enabled    bool   `json:"enabled"`
}

// ProfileReferences reports profiles that bind a rule resource. It keeps the
// rules service independent from profile persistence.
type ProfileReferences interface {
	ReferencesForResource(ctx context.Context, resourceID string) ([]Reference, error)
}

type Reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ReferencedError prevents deletion from producing dangling profile bindings.
type ReferencedError struct {
	ResourceID string
	Profiles   []Reference
}

func (e *ReferencedError) Error() string {
	return fmt.Sprintf("规则资源仍被 %d 个档案引用", len(e.Profiles))
}

// ResourceResponse is the public representation. TemplateKey is deliberately
// absent: it is an installation-only mapping detail.
type ResourceResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Format         Format     `json:"format"`
	UpdateInterval string     `json:"updateInterval"`
	Disabled       bool       `json:"disabled"`
	LocalPath      string     `json:"localPath"`
	LastAttemptAt  *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt,omitempty"`
	LastDigest     string     `json:"lastDigest,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	NextUpdateAt   *time.Time `json:"nextUpdateAt,omitempty"`
}

type UpdateResult struct {
	Changed bool   `json:"changed"`
	Digest  string `json:"digest"`
	Message string `json:"message"`
}
