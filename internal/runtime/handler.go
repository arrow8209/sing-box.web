package runtime

import (
	"errors"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

const connectionsPath = "/api/v1/runtime/connections"
const proxyGroupsPath = "/api/v1/runtime/proxy-groups"
const modePath = "/api/v1/runtime/mode"

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "runtime_unavailable", Message: "运行数据服务不可用"})
		return
	}
	if r.URL.Path == modePath {
		h.mode(w, r)
		return
	}
	if r.URL.Path == proxyGroupsPath {
		h.proxyGroups(w, r)
		return
	}
	if r.URL.Path == connectionsPath {
		switch r.Method {
		case http.MethodGet:
			value, err := h.service.Connections(r.Context())
			if err != nil {
				writeServiceError(w, err)
				return
			}
			httpapi.WriteJSON(w, http.StatusOK, value)
		case http.MethodDelete:
			if err := h.service.CloseAllConnections(r.Context()); err != nil {
				writeServiceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodDelete)
		}
		return
	}
	if !strings.HasPrefix(r.URL.Path, connectionsPath+"/") {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	if r.Method != http.MethodDelete {
		httpapi.MethodNotAllowed(w, http.MethodDelete)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, connectionsPath+"/")
	if strings.Contains(id, "/") {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	if err := h.service.CloseConnection(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) proxyGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := h.service.ProxyGroups(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, groups)
	case http.MethodPut:
		var input ProxySelection
		if err := httpapi.DecodeJSON(w, r, &input, 16<<10); err != nil {
			apierror.WriteError(w, err)
			return
		}
		selection, err := h.service.SelectProxy(r.Context(), input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, selection)
	default:
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (h *Handler) mode(w http.ResponseWriter, r *http.Request) {
	var snapshot ModeSnapshot
	var err error
	switch r.Method {
	case http.MethodGet:
		snapshot, err = h.service.Mode(r.Context())
	case http.MethodPut:
		var input ModeSelection
		if err := httpapi.DecodeJSON(w, r, &input, 1024); err != nil {
			apierror.WriteError(w, err)
			return
		}
		snapshot, err = h.service.SetMode(r.Context(), input)
	default:
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPut)
		return
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, snapshot)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCoreNotRunning):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "core_not_running", Message: "sing-box 核心当前未运行"})
	case errors.Is(err, ErrControllerNotConfigured):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "clash_api_not_configured", Message: "当前应用配置未启用可用的本机 Clash API"})
	case errors.Is(err, ErrInvalidConnectionID):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_connection_id", Message: "连接 ID 无效"})
	case errors.Is(err, ErrConnectionNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "connection_not_found", Message: "连接已不存在"})
	case errors.Is(err, ErrInvalidProxySelection):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_proxy_selection", Message: "策略组或节点名称无效"})
	case errors.Is(err, ErrProxyGroupNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "proxy_group_not_found", Message: "当前运行配置中不存在此策略组"})
	case errors.Is(err, ErrProxyGroupReadOnly):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "proxy_group_read_only", Message: "仅 Selector 策略组支持手动切换，URLTest 由核心自动选择"})
	case errors.Is(err, ErrProxyNodeNotFound):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "proxy_node_not_found", Message: "此节点不在策略组的当前运行配置中"})
	case errors.Is(err, ErrProxySelectionFailed):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "proxy_selection_failed", Message: "核心未确认节点切换，请刷新后重试"})
	case errors.Is(err, ErrInvalidMode):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_runtime_mode", Message: "仅支持规则、全局和直连模式"})
	case errors.Is(err, ErrModeDisabled):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "runtime_mode_unavailable", Message: "当前运行配置未启用此模式，请先配置对应的模式规则"})
	case errors.Is(err, ErrModeChange):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "runtime_mode_change_failed", Message: "核心未确认模式切换，请刷新后重试"})
	default:
		apierror.Write(w, http.StatusBadGateway, apierror.Error{Code: "clash_api_unavailable", Message: "无法访问 sing-box 运行数据"})
	}
}
