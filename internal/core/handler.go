package core

import (
	"context"
	"errors"
	"net/http"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

type Lifecycle interface {
	StartCore(context.Context) error
	RestartCore(context.Context) error
}

type Handler struct {
	manager   *Manager
	lifecycle Lifecycle
}

func NewHandler(manager *Manager, lifecycle ...Lifecycle) *Handler {
	handler := &Handler{manager: manager}
	if len(lifecycle) > 0 {
		handler.lifecycle = lifecycle[0]
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.manager == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "core_unavailable", Message: "核心状态不可用"})
		return
	}
	if r.URL.Path == "/api/v1/core/restart" {
		if r.Method != http.MethodPost {
			httpapi.MethodNotAllowed(w, http.MethodPost)
			return
		}
		h.control(w, true)
		return
	}
	switch r.Method {
	case http.MethodGet:
		httpapi.WriteJSON(w, http.StatusOK, h.manager.Status(r.Context()))
	case http.MethodPost:
		h.control(w, false)
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
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodDelete)
	}
}

func (h *Handler) control(w http.ResponseWriter, restart bool) {
	if h.lifecycle == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "core_control_unavailable", Message: "核心启动服务不可用"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+15*time.Second)
	defer cancel()
	stage := "start_core"
	code := "core_start_failed"
	timeoutCode := "core_start_timeout"
	message := "启动核心失败，请查看核心状态和日志"
	action := h.lifecycle.StartCore
	if restart {
		stage = "restart_core"
		code = "core_restart_failed"
		timeoutCode = "core_restart_timeout"
		message = "重启核心失败，请查看核心状态和日志"
		action = h.lifecycle.RestartCore
	}
	if err := action(ctx); err != nil {
		switch {
		case errors.Is(err, ErrNoAppliedConfig):
			apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_config_not_applied", Message: "请先应用一份可用配置", Stage: stage})
		case errors.Is(err, ErrNotInstalled):
			apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_not_installed", Message: "请先安装 sing-box 核心", Stage: stage})
		case errors.Is(err, ErrAlreadyRunning):
			apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_already_running", Message: "sing-box 核心已经运行", Stage: stage})
		case errors.Is(err, ErrNotRunning):
			apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_not_running", Message: "sing-box 核心当前未运行", Stage: stage})
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			apierror.Write(w, http.StatusGatewayTimeout, apierror.Error{Code: timeoutCode, Message: "等待核心操作完成超时", Stage: stage})
		default:
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: code, Message: message, Stage: stage})
		}
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.manager.Status(context.Background()))
}
