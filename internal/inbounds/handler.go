package inbounds

import (
	"errors"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/storage"
)

const requestLimit = maxInboundBytes + (64 << 10)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "inbound_resources_unavailable", Message: "入站配置服务不可用"})
		return
	}
	base := "/api/v1/inbound-resources"
	if r.URL.Path == base {
		switch r.Method {
		case http.MethodGet:
			h.list(w, r)
		case http.MethodPost:
			h.create(w, r)
		default:
			httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, base+"/")
	if !ok || !inboundIDPattern.MatchString(rest) {
		notFound(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.get(w, r, rest)
	case http.MethodPut:
		h.replace(w, r, rest)
	case http.MethodDelete:
		h.delete(w, r, rest)
	default:
		httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, err, "list_inbound_resources")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	item, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err, "get_inbound_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input Input
	if err := httpapi.DecodeJSON(w, r, &input, requestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	item, err := h.service.Create(r.Context(), input)
	if err != nil {
		writeError(w, err, "create_inbound_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, item)
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request, id string) {
	var input Input
	if err := httpapi.DecodeJSON(w, r, &input, requestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	item, err := h.service.Replace(r.Context(), id, input)
	if err != nil {
		writeError(w, err, "replace_inbound_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeError(w, err, "delete_inbound_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}

func notFound(w http.ResponseWriter) {
	apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "inbound_resource_not_found", Message: "入站配置不存在"})
}

func writeError(w http.ResponseWriter, err error, stage string) {
	if errors.Is(err, ErrNotFound) {
		notFound(w)
		return
	}
	var corrupt *storage.CorruptFileError
	if errors.As(err, &corrupt) {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "managed_state_corrupt", Message: "入站配置状态文件损坏", Stage: stage, Details: map[string]any{"file": corrupt.Path}})
		return
	}
	apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_inbound_resource", Message: err.Error(), Stage: stage})
}
