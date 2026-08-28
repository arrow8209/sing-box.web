package application

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

const applyRequestLimit = 1024

type Handler struct{ coordinator *Coordinator }

type applyBody struct {
	Revision              uint64 `json:"revision"`
	ConfirmedConfigDigest string `json:"confirmedConfigDigest"`
}

func NewHandler(coordinator *Coordinator) *Handler { return &Handler{coordinator: coordinator} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const base = "/api/v1/profiles/"
	rest, ok := strings.CutPrefix(r.URL.Path, base)
	if !ok || !strings.HasSuffix(rest, "/apply") {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	id := strings.TrimSuffix(rest, "/apply")
	if !validID(id) {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	if r.Method != http.MethodPost {
		httpapi.MethodNotAllowed(w, http.MethodPost)
		return
	}
	if h == nil || h.coordinator == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "application_unavailable", Message: "应用服务不可用"})
		return
	}
	var body applyBody
	if err := httpapi.DecodeJSON(w, r, &body, applyRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if body.Revision == 0 {
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_revision", Message: "revision 必须为正整数", Stage: "check_config"})
		return
	}

	// Once an application request has entered the lifecycle transaction, a
	// disconnected browser must not strand the core between Stop and recovery.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := h.coordinator.Apply(ctx, ApplyRequest{ProfileID: id, Revision: body.Revision, ConfirmedConfigDigest: body.ConfirmedConfigDigest})
	if err != nil {
		h.writeApplyError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) writeApplyError(w http.ResponseWriter, err error) {
	var conflict profile.RevisionConflict
	switch {
	case errors.As(err, &conflict):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "draft_revision_conflict", Message: "服务器草稿已被其他页面更新", Stage: "check_config", LatestRevision: conflict.LatestRevision})
	case errors.Is(err, profile.ErrNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "profile_not_found", Message: "档案不存在", Stage: "check_config"})
	case errors.Is(err, profile.ErrQuarantined):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "profile_quarantined", Message: "档案草稿已隔离", Stage: "check_config"})
	case errors.Is(err, rules.ErrNotFound):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "rule_resource_not_found", Message: "规则资源不存在", Stage: "check_config"})
	case errors.Is(err, rules.ErrResourceNotDownloaded), errors.Is(err, rules.ErrResourceFileDrift):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "rule_resource_unavailable", Message: "规则资源尚不可用，请先更新网络规则", Stage: "check_config"})
	case errors.Is(err, ErrExternalDrift):
		digest, _ := DigestDirectory(h.coordinator.core.Layout().ConfigDir)
		apierror.Write(w, http.StatusConflict, apierror.Error{
			Code: "config_external_drift", Message: "正式配置已在主机侧变更，请确认后重试", Stage: "swap_config",
			Details: map[string]string{"confirmationDigest": digest},
		})
	case errors.Is(err, ErrStartFailed):
		apierror.Write(w, http.StatusBadGateway, apierror.Error{Code: "core_start_failed", Message: "新配置启动失败，已尝试恢复此前配置和核心", Stage: "start_core"})
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		apierror.Write(w, http.StatusGatewayTimeout, apierror.Error{Code: "application_timeout", Message: "配置应用超时，正在以服务器状态为准恢复", Stage: "recover"})
	default:
		apierror.WriteError(w, err)
	}
}

func validID(value string) bool {
	return len(value) == 32 && strings.Trim(value, "0123456789abcdef") == ""
}
