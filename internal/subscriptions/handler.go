package subscriptions

import (
	"errors"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/storage"
)

const requestLimit = maxSubscriptionSize + (64 << 10)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "subscriptions_unavailable", Message: "订阅服务不可用"})
		return
	}
	base := "/api/v1/subscriptions"
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
	if !ok || !subscriptionIDPattern.MatchString(rest) {
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
		writeError(w, err, "list_subscriptions")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, items)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, id string) {
	item, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err, "get_subscription")
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
		writeError(w, err, "create_subscription")
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
		writeError(w, err, "replace_subscription")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeError(w, err, "delete_subscription")
		return
	}
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}

func notFound(w http.ResponseWriter) {
	apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "subscription_not_found", Message: "订阅不存在"})
}

func writeError(w http.ResponseWriter, err error, stage string) {
	if errors.Is(err, ErrNotFound) {
		notFound(w)
		return
	}
	var corrupt *storage.CorruptFileError
	if errors.As(err, &corrupt) {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "managed_state_corrupt", Message: "订阅状态文件损坏", Stage: stage, Details: map[string]any{"file": corrupt.Path}})
		return
	}
	apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_subscription", Message: err.Error(), Stage: stage})
}
