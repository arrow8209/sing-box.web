package rules

import (
	"errors"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/storage"
)

const requestLimit = 16 << 10

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "rule_resources_unavailable", Message: "规则资源服务不可用"})
		return
	}
	base := "/api/v1/rule-resources"
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
	part := strings.TrimPrefix(r.URL.Path, base+"/")
	parts := strings.Split(part, "/")
	if len(parts) == 0 || !resourceIDPattern.MatchString(parts[0]) {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "rule_resource_not_found", Message: "规则资源不存在"})
		return
	}
	if len(parts) == 2 && parts[1] == "references" && r.Method == http.MethodGet {
		h.references(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "update" && r.Method == http.MethodPost {
		h.update(w, r, parts[0])
		return
	}
	if len(parts) != 1 {
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		return
	}
	switch r.Method {
	case http.MethodPatch:
		h.patch(w, r, parts[0])
	case http.MethodDelete:
		h.delete(w, r, parts[0])
	default:
		httpapi.MethodNotAllowed(w, http.MethodPatch, http.MethodDelete)
	}
}
func (h *Handler) update(w http.ResponseWriter, r *http.Request, id string) {
	result, err := h.service.UpdateNow(r.Context(), id)
	if err != nil {
		writeError(w, err, "update_rule_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	resources, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, err, "list_rule_resources")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resources)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	if err := httpapi.DecodeJSON(w, r, &input, requestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	resource, err := h.service.Create(r.Context(), input)
	if err != nil {
		writeError(w, err, "create_rule_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, resource)
}
func (h *Handler) patch(w http.ResponseWriter, r *http.Request, id string) {
	var input PatchInput
	if err := httpapi.DecodeJSON(w, r, &input, requestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	resource, err := h.service.Patch(r.Context(), id, input)
	if err != nil {
		writeError(w, err, "patch_rule_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resource)
}
func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeError(w, err, "delete_rule_resource")
		return
	}
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}
func (h *Handler) references(w http.ResponseWriter, r *http.Request, id string) {
	references, err := h.service.References(r.Context(), id)
	if err != nil {
		writeError(w, err, "rule_resource_references")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, struct {
		Profiles []Reference `json:"profiles"`
	}{Profiles: references})
}

func writeError(w http.ResponseWriter, err error, stage string) {
	var referenced *ReferencedError
	switch {
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "rule_resource_not_found", Message: "规则资源不存在", Stage: stage})
	case errors.As(err, &referenced):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "rule_resource_referenced", Message: "规则资源仍被档案引用", Stage: stage, Details: map[string]any{"profiles": referenced.Profiles}})
	default:
		var corrupt *storage.CorruptFileError
		if errors.As(err, &corrupt) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "managed_state_corrupt", Message: "规则资源状态文件损坏", Stage: stage, Details: map[string]any{"file": corrupt.Path}})
			return
		}
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_rule_resource", Message: err.Error(), Stage: stage})
	}
}
