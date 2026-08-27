package application

import (
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

// Handler intentionally leaves application unavailable until its lifecycle,
// rollback and ownership semantics have been agreed on.
type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

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
	apierror.Write(w, http.StatusNotImplemented, apierror.Error{Code: "application_not_implemented", Message: "应用逻辑尚未实现"})
}

func validID(value string) bool {
	return len(value) == 32 && strings.Trim(value, "0123456789abcdef") == ""
}
