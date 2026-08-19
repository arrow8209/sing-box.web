package validation

import "time"

type Issue struct {
	Stage   string `json:"stage"`
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Valid            bool      `json:"valid"`
	Issues           []Issue   `json:"issues"`
	TargetVersion    string    `json:"targetVersion"`
	InstalledVersion string    `json:"installedVersion"`
	CheckedAt        time.Time `json:"checkedAt"`
}
