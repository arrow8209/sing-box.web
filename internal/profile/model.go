package profile

type RuleBinding struct {
	ResourceID string `json:"resourceId"`
	Tag        string `json:"tag"`
	Enabled    bool   `json:"enabled"`
}
