package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/configgen"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/profile"
)

type Handler struct{ coordinator *Coordinator }
type applyBody struct {
	Revision              uint64 `json:"revision"`
	ConfirmedConfigDigest string `json:"confirmedConfigDigest"`
}

func NewHandler(coordinator *Coordinator) *Handler { return &Handler{coordinator: coordinator} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.coordinator == nil {
		apierror.Write(w, 500, apierror.Error{Code: "application_unavailable", Message: "应用服务不可用"})
		return
	}
	if r.Method != http.MethodPost {
		httpapi.MethodNotAllowed(w, http.MethodPost)
		return
	}
	const base = "/api/v1/profiles/"
	rest, ok := strings.CutPrefix(r.URL.Path, base)
	if !ok || !strings.HasSuffix(rest, "/apply") {
		apierror.Write(w, 404, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	id := strings.TrimSuffix(rest, "/apply")
	if !validID(id) {
		apierror.Write(w, 404, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	var body applyBody
	if err := httpapi.DecodeJSON(w, r, &body, 1024); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if body.Revision == 0 {
		apierror.Write(w, 400, apierror.Error{Code: "invalid_revision", Message: "revision 必须为正整数", Stage: "snapshot"})
		return
	}
	// A disconnected client must not cancel a transaction after stop_core.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := h.coordinator.Apply(ctx, ApplyRequest{ProfileID: id, Revision: body.Revision, ConfirmedConfigDigest: body.ConfirmedConfigDigest})
	if err == nil {
		httpapi.WriteJSON(w, http.StatusOK, result)
		return
	}
	h.writeApplyError(w, err)
}

func validID(value string) bool {
	return len(value) == 32 && strings.Trim(value, "0123456789abcdef") == ""
}
func (h *Handler) writeApplyError(w http.ResponseWriter, err error) {
	var revision profile.RevisionConflict
	var checkFailure *CheckFailure
	switch {
	case errors.As(err, &revision):
		apierror.Write(w, 409, apierror.Error{Code: "draft_revision_conflict", Message: "服务器草稿已被其他页面更新", Stage: "snapshot", LatestRevision: revision.LatestRevision})
	case errors.Is(err, ErrExternalDrift):
		digest, _ := DigestDirectory(h.coordinator.core.Layout().ConfigDir)
		apierror.Write(w, 409, apierror.Error{Code: "config_external_drift", Message: "正式配置已在主机侧变更，请确认后重试", Stage: "snapshot", Details: map[string]string{"confirmationDigest": digest}})
	case errors.As(err, &checkFailure):
		apierror.Write(w, 422, apierror.Error{Code: "config_check_failed", Message: checkFailureMessage(checkFailure), Stage: "check_config", Details: newCheckFailureDetails(checkFailure)})
	case errors.Is(err, ErrCheckFailed):
		apierror.Write(w, 422, apierror.Error{Code: "config_check_failed", Message: "配置检查失败", Stage: "check_config"})
	case errors.Is(err, ErrStartFailed):
		apierror.Write(w, 502, apierror.Error{Code: "core_start_failed", Message: "核心启动失败，已尝试恢复旧配置", Stage: "start_core"})
	default:
		apierror.WriteError(w, err)
	}
}

type checkFailureDetails struct {
	Command                []string `json:"command"`
	CheckedConfigDirectory string   `json:"checkedConfigDirectory"`
	ReviewCommand          []string `json:"reviewCommand,omitempty"`
	ReviewConfigDirectory  string   `json:"reviewConfigDirectory,omitempty"`
	ConfigFiles            []string `json:"configFiles"`
	ProfileID              string   `json:"profileId"`
	Revision               uint64   `json:"revision"`
	ConfigDigest           string   `json:"configDigest"`
	ExitCode               int      `json:"exitCode"`
	TimedOut               bool     `json:"timedOut"`
	OutputTruncated        bool     `json:"outputTruncated"`
	Stdout                 string   `json:"stdout,omitempty"`
	Stderr                 string   `json:"stderr,omitempty"`
	ProcessError           string   `json:"processError,omitempty"`
	ReviewError            string   `json:"reviewError,omitempty"`
}

func newCheckFailureDetails(failure *CheckFailure) checkFailureDetails {
	command, _ := configgen.CheckCommand(failure.WorkDir, failure.BinaryPath, failure.CheckedConfigDirectory)
	details := checkFailureDetails{
		Command:                command,
		CheckedConfigDirectory: checkedDirectory(command),
		ConfigFiles:            append([]string(nil), failure.ConfigFiles...),
		ProfileID:              failure.ProfileID,
		Revision:               failure.Revision,
		ConfigDigest:           failure.ConfigDigest,
		ExitCode:               failure.Result.ExitCode,
		TimedOut:               failure.Result.TimedOut,
		OutputTruncated:        failure.Result.OutputTruncated,
		Stdout:                 redactWorkDirectory(failure.Result.Stdout, failure.WorkDir),
		Stderr:                 redactWorkDirectory(failure.Result.Stderr, failure.WorkDir),
	}
	if failure.ReviewConfigDirectory != "" {
		details.ReviewCommand, _ = configgen.CheckCommand(failure.WorkDir, failure.BinaryPath, failure.ReviewConfigDirectory)
		details.ReviewConfigDirectory = checkedDirectory(details.ReviewCommand)
	}
	if failure.Cause != nil {
		details.ProcessError = checkProcessError(failure)
	}
	if failure.ReviewError != nil {
		details.ReviewError = "未能保留失败配置审核副本"
	}
	return details
}

func checkedDirectory(command []string) string {
	if len(command) != 4 {
		return ""
	}
	return command[3]
}

func checkProcessError(failure *CheckFailure) string {
	switch {
	case errors.Is(failure.Cause, configgen.ErrCoreNotInstalled):
		return "sing-box 核心不可用"
	case failure.Result.TimedOut, errors.Is(failure.Cause, configgen.ErrCheckTimeout):
		return "sing-box check 执行超时"
	case failure.Result.ExitCode >= 0:
		return fmt.Sprintf("exit status %d", failure.Result.ExitCode)
	default:
		return "sing-box check 执行失败"
	}
}

func redactWorkDirectory(value, workDir string) string {
	if value == "" || workDir == "" {
		return value
	}
	clean := strings.TrimRight(filepath.Clean(workDir), string(filepath.Separator))
	if clean == "" || clean == "." || !filepath.IsAbs(clean) {
		return value
	}
	value = strings.ReplaceAll(value, clean+string(filepath.Separator), "")
	value = strings.ReplaceAll(value, filepath.ToSlash(clean)+"/", "")
	return strings.ReplaceAll(value, clean, ".")
}

func checkFailureMessage(failure *CheckFailure) string {
	switch {
	case errors.Is(failure.Cause, configgen.ErrCoreNotInstalled):
		return "尚未安装 sing-box 核心"
	case failure.Result.TimedOut, errors.Is(failure.Cause, configgen.ErrCheckTimeout):
		return "sing-box 配置检查超时"
	case failure.Result.ExitCode >= 0:
		return fmt.Sprintf("sing-box 配置检查失败（退出码 %d）", failure.Result.ExitCode)
	default:
		return "无法执行 sing-box 配置检查"
	}
}
