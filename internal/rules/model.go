package rules

import "time"

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
