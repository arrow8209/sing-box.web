package core

import (
	"context"
	"errors"
	"net/http"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

type Handler struct{ manager *Manager }

func NewHandler(manager *Manager) *Handler { return &Handler{manager: manager} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.manager == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "core_unavailable", Message: "核心状态不可用"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		httpapi.WriteJSON(w, http.StatusOK, h.manager.Status(r.Context()))
	case http.MethodDelete:
		// Once accepted, a disconnected browser must not leave the managed
		// process half-stopped. The bound includes waiting for another core
		// transaction plus the graceful and forced termination windows.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+15*time.Second)
		defer cancel()
		if err := h.manager.RequestStop(ctx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				apierror.Write(w, http.StatusGatewayTimeout, apierror.Error{Code: "core_stop_timeout", Message: "等待核心进程结束超时"})
				return
			}
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "core_stop_failed", Message: "停止核心进程失败"})
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.manager.Status(context.Background()))
	default:
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodDelete)
	}
}
