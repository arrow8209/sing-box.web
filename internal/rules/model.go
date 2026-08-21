package rules

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
	ID             string `json:"id"`
	TemplateKey    string `json:"templateKey,omitempty"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         Format `json:"format"`
	UpdateInterval string `json:"updateInterval"`
	Disabled       bool   `json:"disabled"`
	LocalPath      string `json:"localPath"`
}
