package configgen

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

const validationRequestLimit = 1024

var ErrSemanticValidation = errors.New("semantic_validation_failed")

type revisionRequest struct {
	Revision uint64 `json:"revision"`
}

// PreviewResult keeps generated files at the top level so clients can render
// them without understanding the internal Output type.
type PreviewResult struct {
	Revision        uint64            `json:"revision"`
	Files           []File            `json:"files,omitempty"`
	Digest          string            `json:"digest,omitempty"`
	ResourceDigests map[string]string `json:"resourceDigests,omitempty"`
	Errors          []apierror.Error  `json:"errors,omitempty"`
}

type ValidateResult struct {
	PreviewResult
	Valid           bool     `json:"valid"`
	Stage           string   `json:"stage"`
	Message         string   `json:"message,omitempty"`
	Code            string   `json:"code,omitempty"`
	Stdout          string   `json:"stdout,omitempty"`
	Stderr          string   `json:"stderr,omitempty"`
	ExitCode        int      `json:"exitCode"`
	TimedOut        bool     `json:"timedOut,omitempty"`
	OutputTruncated bool     `json:"outputTruncated,omitempty"`
	Command         []string `json:"command,omitempty"`
	ConfigDirectory string   `json:"configDirectory,omitempty"`
}

// Handler coordinates a read-only preview and a full core check whose generated
// candidate remains in config.staging-<profileID>. It has no core lifecycle
// dependency and never writes layout.ConfigDir.
type Handler struct {
	layout    paths.Layout
	profiles  *profile.Service
	rules     *rules.Service
	generator *Generator
	checker   *Checker
	gate      OperationGate
	clock     func() time.Time
}

func NewHandler(layout paths.Layout, profiles *profile.Service, resources *rules.Service, generator *Generator, checker *Checker, gate OperationGate, clock func() time.Time) *Handler {
	if generator == nil {
		generator = NewGenerator()
	}
	if checker == nil {
		checker = NewChecker(layout.WorkDir, nil, 30*time.Second, 64<<10)
	}
	if gate == nil {
		gate = NewOperationGate()
	}
	if clock == nil {
		clock = time.Now
	}
	return &Handler{layout: layout, profiles: profiles, rules: resources, generator: generator, checker: checker, gate: gate, clock: clock}
}

func (h *Handler) Preview(ctx context.Context, profileID string, revision uint64) (PreviewResult, error) {
	draft, snapshot, release, err := h.loadSnapshot(ctx, profileID, revision)
	if err != nil {
		return PreviewResult{}, err
	}
	defer release()
	semantic := ValidateSemantic(draft, snapshot)
	if len(semantic) != 0 {
		return PreviewResult{Revision: revision, Errors: semantic}, ErrSemanticValidation
	}
	output, err := h.generator.Generate(ctx, draft, snapshot)
	if err != nil {
		return PreviewResult{}, err
	}
	return previewResult(revision, output), nil
}

func (h *Handler) Validate(ctx context.Context, profileID string, revision uint64) (result ValidateResult, err error) {
	releaseOperation, err := h.gate.EnterGate(ctx, "validate")
	if err != nil {
		return ValidateResult{}, err
	}
	defer releaseOperation()

	// Record every validation conclusion. RecordValidation itself guards against
	// a late process result overwriting a newer draft revision.
	defer func() {
		if result.Stage == "" {
			return
		}
		_ = h.profiles.RecordValidation(context.Background(), profileID, revision, profile.ValidationSummary{
			Valid: result.Valid, Stage: result.Stage, Message: result.Message, CheckedAt: h.clock(),
		})
	}()

	draft, snapshot, release, err := h.loadSnapshot(ctx, profileID, revision)
	if err != nil {
		return ValidateResult{}, err
	}
	defer release()
	semantic := ValidateSemantic(draft, snapshot)
	if len(semantic) != 0 {
		return ValidateResult{PreviewResult: PreviewResult{Revision: revision, Errors: semantic}, Stage: semanticStage, Code: "semantic_validation_failed", Message: "配置语义检查失败"}, nil
	}
	output, err := h.generator.Generate(ctx, draft, snapshot)
	if err != nil {
		return ValidateResult{}, err
	}
	result = ValidateResult{PreviewResult: previewResult(revision, output), Stage: "core_check"}
	staging, err := h.layout.StagingConfigDir(profileID)
	if err != nil {
		return ValidateResult{}, err
	}
	if err := h.generator.ReplaceStagingDirectory(ctx, output, staging); err != nil {
		return ValidateResult{}, err
	}
	result.Command, _ = CheckCommand(h.layout.WorkDir, h.layout.CoreBinary, staging)
	if len(result.Command) == 4 {
		result.ConfigDirectory = result.Command[3]
	}
	check, checkErr := h.checker.Check(ctx, h.layout.CoreBinary, staging)
	result.Stdout, result.Stderr = check.Stdout, check.Stderr
	result.ExitCode, result.TimedOut, result.OutputTruncated = check.ExitCode, check.TimedOut, check.OutputTruncated
	if checkErr == nil {
		result.Valid, result.Stage, result.Message = true, "core_check", "配置检查通过"
		return result, nil
	}
	switch {
	case errors.Is(checkErr, ErrCoreNotInstalled):
		result.Code, result.Message = "core_not_installed", "尚未安装 sing-box 核心"
	case errors.Is(checkErr, ErrCheckTimeout):
		result.Code, result.Stage, result.Message = "singbox_check_timeout", "core_check", "sing-box 配置检查超时"
	default:
		result.Code, result.Message = "singbox_check_failed", "sing-box 配置检查失败"
	}
	return result, nil
}

func (h *Handler) loadSnapshot(ctx context.Context, profileID string, revision uint64) (profile.Draft, rules.Snapshot, func(), error) {
	if h == nil || h.profiles == nil || h.rules == nil {
		return profile.Draft{}, rules.Snapshot{}, func() {}, errors.New("配置服务不可用")
	}
	draft, _, err := h.profiles.GetDraft(ctx, profileID)
	if err != nil {
		return profile.Draft{}, rules.Snapshot{}, func() {}, err
	}
	if draft.Revision != revision {
		return profile.Draft{}, rules.Snapshot{}, func() {}, profile.RevisionConflict{LatestRevision: draft.Revision}
	}
	snapshot, release, err := h.rules.SnapshotForBindings(ctx, draft.Bindings)
	if err != nil {
		return profile.Draft{}, rules.Snapshot{}, func() {}, err
	}
	return draft, snapshot, release, nil
}

func previewResult(revision uint64, output Output) PreviewResult {
	return PreviewResult{Revision: revision, Files: output.Files, Digest: output.Digest, ResourceDigests: output.ResourceDigests}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.profiles == nil || h.rules == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "configuration_unavailable", Message: "配置服务不可用"})
		return
	}
	const base = "/api/v1/profiles/"
	rest, ok := strings.CutPrefix(r.URL.Path, base)
	parts := strings.Split(rest, "/")
	if !ok || len(parts) != 2 || !validID(parts[0]) || (parts[1] != "preview" && parts[1] != "validate") {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	if r.Method != http.MethodPost {
		httpapi.MethodNotAllowed(w, http.MethodPost)
		return
	}
	var request revisionRequest
	if err := httpapi.DecodeJSON(w, r, &request, validationRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if request.Revision == 0 {
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_revision", Message: "revision 必须为正整数", Stage: parts[1]})
		return
	}
	if parts[1] == "preview" {
		result, err := h.Preview(r.Context(), parts[0], request.Revision)
		if errors.Is(err, ErrSemanticValidation) {
			httpapi.WriteJSON(w, http.StatusOK, result)
			return
		}
		if err != nil {
			h.writeError(w, err, "preview")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
		return
	}
	result, err := h.Validate(r.Context(), parts[0], request.Revision)
	if err != nil {
		h.writeError(w, err, "validate")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) writeError(w http.ResponseWriter, err error, stage string) {
	var conflict profile.RevisionConflict
	switch {
	case errors.As(err, &conflict):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "draft_revision_conflict", Message: "服务器草稿已被其他页面更新", Stage: stage, LatestRevision: conflict.LatestRevision})
	case errors.Is(err, profile.ErrNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "profile_not_found", Message: "档案不存在", Stage: stage})
	case errors.Is(err, rules.ErrNotFound):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "rule_resource_not_found", Message: "规则资源不存在", Stage: stage})
	case errors.Is(err, rules.ErrResourceNotDownloaded), errors.Is(err, rules.ErrResourceFileDrift):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "rule_resource_unavailable", Message: "规则资源尚不可用", Stage: stage})
	default:
		apierror.WriteError(w, err)
	}
}

func validID(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
