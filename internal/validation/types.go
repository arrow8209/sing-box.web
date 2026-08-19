package validation

import (
	"sort"
	"time"
)

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

func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(left, right int) bool {
		if issues[left].Path != issues[right].Path {
			return issues[left].Path < issues[right].Path
		}
		return issues[left].Code < issues[right].Code
	})
}
