package core

import (
	"net/http"

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
	if r.Method != http.MethodGet {
		httpapi.MethodNotAllowed(w, http.MethodGet)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.manager.Status(r.Context()))
}
