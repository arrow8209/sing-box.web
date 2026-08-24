package settings

import (
	"errors"
	"net/http"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

const proxySettingsRequestLimit = maxProxyURLBytes + 1024

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "settings_unavailable", Message: "设置服务不可用"})
		return
	}
	if r.URL.Path != "/api/v1/settings/proxy" {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		value, err := h.store.Get()
		if err != nil {
			apierror.WriteError(w, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, value)
	case http.MethodPut:
		var input ProxySettings
		if err := httpapi.DecodeJSON(w, r, &input, proxySettingsRequestLimit); err != nil {
			apierror.WriteError(w, err)
			return
		}
		value, err := h.store.UpdateProxy(input.ProxyURL)
		if errors.Is(err, ErrInvalidProxyURL) {
			apierror.WriteError(w, apierror.Field("invalid_proxy_url", "代理地址必须是无账号密码的 HTTP 或 HTTPS URL", "save_proxy_settings", "proxyUrl", err))
			return
		}
		if err != nil {
			apierror.WriteError(w, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, value)
	default:
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}
