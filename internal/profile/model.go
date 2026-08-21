package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Meta struct {
	ID                     string             `json:"id"`
	Name                   string             `json:"name"`
	DraftRevision          uint64             `json:"draftRevision"`
	AppliedRevision        *uint64            `json:"appliedRevision"`
	AppliedDigest          string             `json:"appliedDigest"`
	AppliedResourceDigests map[string]string  `json:"appliedResourceDigests"`
	AppliedAt              *time.Time         `json:"appliedAt"`
	NeedsApply             bool               `json:"needsApply"`
	LastValidation         *ValidationSummary `json:"lastValidation"`
	QuarantinedError       string             `json:"quarantinedError,omitempty"`
}

type ValidationSummary struct {
	Valid     bool      `json:"valid"`
	Revision  uint64    `json:"revision"`
	CheckedAt time.Time `json:"checkedAt"`
	Stage     string    `json:"stage"`
	Message   string    `json:"message"`
}

type RuleBinding struct {
	ResourceID string `json:"resourceId"`
	Tag        string `json:"tag"`
	Enabled    bool   `json:"enabled"`
}

type AppliedResult struct {
	ProfileID       string
	Revision        uint64
	Digest          string
	ResourceDigests map[string]string
	AppliedAt       time.Time
	Validation      ValidationSummary
}

type Reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DefaultBindingSource interface {
	DefaultBindings(ctx context.Context) ([]RuleBinding, error)
}

// DeleteGuard reserves deletion of a currently applied profile for the core
// operations layer. The foundational service installs a conservative guard
// until that layer injects one that can identify the active profile.
type DeleteGuard interface {
	GuardDelete(ctx context.Context, profileID string, stopCurrent bool) error
}

type Draft struct {
	ProfileID string                     `json:"profileId"`
	Revision  uint64                     `json:"revision"`
	Config    map[string]json.RawMessage `json:"config"`
	Buffers   map[string]string          `json:"buffers"`
	Bindings  []RuleBinding              `json:"bindings"`
	SavedAt   time.Time                  `json:"savedAt"`
}

type DraftInput struct {
	Buffers  map[string]string `json:"buffers"`
	Bindings []RuleBinding     `json:"bindings"`
}

type CreateInput struct {
	Name string `json:"name"`
}
type RenameInput struct {
	Name string `json:"name"`
}
type DeleteInput struct {
	ConfirmName string `json:"confirmName"`
	StopCurrent bool   `json:"stopCurrent"`
}

var (
	ErrNotFound                           = fmt.Errorf("档案不存在")
	ErrPreconditionRequired               = fmt.Errorf("保存草稿必须携带有效 If-Match")
	ErrQuarantined                        = fmt.Errorf("档案草稿已隔离")
	ErrDeleteNameMismatch                 = fmt.Errorf("删除确认名称不匹配")
	ErrCurrentProfileRequiresCoreDecision = fmt.Errorf("当前档案需要核心操作层决策")
)

type RevisionConflict struct{ LatestRevision uint64 }

func (e RevisionConflict) Error() string {
	return fmt.Sprintf("草稿修订冲突，当前修订为 %d", e.LatestRevision)
}

func RevisionETag(revision uint64) string { return fmt.Sprintf(`"rev-%d"`, revision) }

func ParseRevisionETag(value string) (uint64, error) {
	if len(value) < len(`"rev-1"`) || !strings.HasPrefix(value, `"rev-`) || !strings.HasSuffix(value, `"`) {
		return 0, fmt.Errorf("无效草稿修订 ETag")
	}
	number := value[len(`"rev-`) : len(value)-1]
	if number == "" || (len(number) > 1 && number[0] == '0') {
		return 0, fmt.Errorf("无效草稿修订 ETag")
	}
	revision, err := strconv.ParseUint(number, 10, 64)
	if err != nil || revision == 0 {
		return 0, fmt.Errorf("无效草稿修订 ETag")
	}
	return revision, nil
}
