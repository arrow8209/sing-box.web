package runtime

import (
	"errors"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

const connectionsPath = "/api/v1/runtime/connections"

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "runtime_unavailable", Message: "连接服务不可用"})
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
	default:
		apierror.Write(w, http.StatusBadGateway, apierror.Error{Code: "clash_api_unavailable", Message: "无法读取 sing-box 连接数据"})
	}
}
