package updater

import (
	"errors"
	"net/http"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, 500, apierror.Error{Code: "updater_unavailable", Message: "核心更新服务不可用"})
		return
	}
	switch r.URL.Path {
	case "/api/v1/core/releases/latest":
		if r.Method != http.MethodGet {
			httpapi.MethodNotAllowed(w, http.MethodGet)
			return
		}
		release, err := h.service.Latest(r.Context())
		if err != nil {
			apierror.Write(w, 502, apierror.Error{Code: "release_lookup_failed", Message: "无法获取官方最新版本"})
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, struct {
			Version string `json:"version"`
			Arch    string `json:"arch"`
		}{release.Version, release.Arch})
	case "/api/v1/core/update":
		if r.Method == http.MethodGet {
			httpapi.WriteJSON(w, http.StatusOK, h.service.Status())
			return
		}
		if r.Method == http.MethodDelete {
			id, err := h.service.Cancel()
			if errors.Is(err, ErrNoUpdateInProgress) {
				apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_update_not_running", Message: "当前没有正在下载的核心"})
				return
			}
			if errors.Is(err, ErrUpdateNotCancellable) {
				apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_update_not_cancellable", Message: "当前阶段无法停止下载"})
				return
			}
			if err != nil {
				apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "core_update_cancel_failed", Message: "停止核心下载失败"})
				return
			}
			httpapi.WriteJSON(w, http.StatusAccepted, struct {
				OperationID string `json:"operationId"`
			}{id})
			return
		}
		if r.Method != http.MethodPost {
			httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodDelete)
			return
		}
		var confirmation UpdateConfirmation
		if err := httpapi.DecodeJSON(w, r, &confirmation, 1024); err != nil {
			apierror.WriteError(w, err)
			return
		}
		id, err := h.service.Start(r.Context(), confirmation)
		if errors.Is(err, ErrUpdateInProgress) {
			apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_update_in_progress", Message: "已有核心更新正在进行", Stage: "waiting_for_lock"})
			return
		}
		if err != nil {
			apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "update_confirmation_required", Message: "必须确认目标版本"})
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, struct {
			OperationID string `json:"operationId"`
		}{id})
	default:
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
	}
}
