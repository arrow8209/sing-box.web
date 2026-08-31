package profile

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

const profileRequestLimit = 16 << 10

type createRequest struct {
	Name string `json:"name"`
}

type renameRequest struct {
	Name string `json:"name"`
}

type deleteRequest struct {
	ConfirmName string `json:"confirmName"`
	StopCurrent bool   `json:"stopCurrent"`
}

type DraftSaveOutcome struct {
	Attempted bool   `json:"attempted"`
	Applied   bool   `json:"applied"`
	Reloaded  bool   `json:"reloaded"`
	Message   string `json:"message"`
}

type DraftSavedHook func(context.Context, string, uint64) *DraftSaveOutcome

// Handler serves profile CRUD and draft endpoints. Authentication and CSRF are
// intentionally supplied by the top-level router, keeping this package free of
// auth dependencies.
type Handler struct {
	service    *Service
	draftSaved DraftSavedHook
}

func NewHandler(service *Service, hooks ...DraftSavedHook) *Handler {
	handler := &Handler{service: service}
	if len(hooks) > 0 {
		handler.draftSaved = hooks[0]
	}
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "profile_unavailable", Message: "档案服务不可用"})
		return
	}

	base := "/api/v1/profiles"
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
	if !ok {
		notFound(w)
		return
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || !validProfileID(parts[0]) {
		notFound(w)
		return
	}
	profileID := parts[0]
	switch {
	case len(parts) == 1:
		switch r.Method {
		case http.MethodPatch:
			h.rename(w, r, profileID)
		case http.MethodDelete:
			h.delete(w, r, profileID)
		default:
			httpapi.MethodNotAllowed(w, http.MethodPatch, http.MethodDelete)
		}
	case len(parts) == 2 && parts[1] == "draft":
		switch r.Method {
		case http.MethodGet:
			h.getDraft(w, r, profileID)
		case http.MethodPut:
			h.putDraft(w, r, profileID)
		default:
			httpapi.MethodNotAllowed(w, http.MethodGet, http.MethodPut)
		}
	case len(parts) == 2 && parts[1] == "copy":
		if r.Method != http.MethodPost {
			httpapi.MethodNotAllowed(w, http.MethodPost)
			return
		}
		h.duplicate(w, r, profileID)
	default:
		notFound(w)
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.service.List(r.Context())
	if err != nil {
		writeProfileError(w, err, "list_profiles")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, profiles)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var request createRequest
	if err := httpapi.DecodeJSON(w, r, &request, profileRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if _, err := validateName(request.Name); err != nil {
		invalidName(w, err)
		return
	}
	meta, _, err := h.service.Create(r.Context(), CreateInput{Name: request.Name})
	if err != nil {
		writeProfileError(w, err, "create_profile")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, meta)
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request, profileID string) {
	var request renameRequest
	if err := httpapi.DecodeJSON(w, r, &request, profileRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if _, err := validateName(request.Name); err != nil {
		invalidName(w, err)
		return
	}
	meta, err := h.service.Rename(r.Context(), profileID, RenameInput{Name: request.Name})
	if err != nil {
		writeProfileError(w, err, "rename_profile")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, meta)
}

func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request, profileID string) {
	meta, _, err := h.service.Duplicate(r.Context(), profileID)
	if err != nil {
		writeProfileError(w, err, "copy_profile")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, meta)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, profileID string) {
	var request deleteRequest
	if err := httpapi.DecodeJSON(w, r, &request, profileRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), profileID, DeleteInput{ConfirmName: request.ConfirmName, StopCurrent: request.StopCurrent}); err != nil {
		writeProfileError(w, err, "delete_profile")
		return
	}
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) getDraft(w http.ResponseWriter, r *http.Request, profileID string) {
	draft, etag, err := h.service.GetDraft(r.Context(), profileID)
	if err != nil {
		writeProfileError(w, err, "get_draft")
		return
	}
	w.Header().Set("ETag", etag)
	httpapi.WriteJSON(w, http.StatusOK, draft)
}

func (h *Handler) putDraft(w http.ResponseWriter, r *http.Request, profileID string) {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		apierror.Write(w, http.StatusPreconditionRequired, apierror.Error{Code: "if_match_required", Message: "保存草稿必须携带 If-Match", Stage: "save_draft"})
		return
	}
	var input DraftInput
	if err := httpapi.DecodeJSON(w, r, &input, 4<<20); err != nil {
		apierror.WriteError(w, err)
		return
	}
	draft, etag, err := h.service.SaveDraft(r.Context(), profileID, ifMatch, input)
	var conflict RevisionConflict
	if errors.As(err, &conflict) {
		w.Header().Set("ETag", RevisionETag(conflict.LatestRevision))
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "draft_revision_conflict", Message: "服务器草稿已被其他页面更新", Stage: "save_draft", LatestRevision: conflict.LatestRevision})
		return
	}
	if err != nil {
		writeProfileError(w, err, "save_draft")
		return
	}
	w.Header().Set("ETag", etag)
	var outcome *DraftSaveOutcome
	if h.draftSaved != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute+15*time.Second)
		outcome = h.draftSaved(ctx, profileID, draft.Revision)
		cancel()
	}
	httpapi.WriteJSON(w, http.StatusOK, struct {
		Draft
		AutoReload *DraftSaveOutcome `json:"autoReload,omitempty"`
	}{Draft: draft, AutoReload: outcome})
}

func validProfileID(value string) bool {
	if len(value) != 32 {
		return false
	}
	decoded := make([]byte, 16)
	if _, err := hex.Decode(decoded, []byte(value)); err != nil {
		return false
	}
	return strings.ToLower(value) == value
}

func invalidName(w http.ResponseWriter, err error) {
	apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_profile_name", Message: err.Error(), Stage: "profile"})
}

func notFound(w http.ResponseWriter) {
	apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
}

func writeProfileError(w http.ResponseWriter, err error, stage string) {
	switch {
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "profile_not_found", Message: "档案不存在", Stage: stage})
	case errors.Is(err, ErrQuarantined):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "profile_quarantined", Message: "档案草稿已隔离", Stage: stage})
	case errors.Is(err, ErrDeleteNameMismatch):
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "delete_confirmation_mismatch", Message: "删除确认名称不匹配", Stage: stage})
	case errors.Is(err, ErrCurrentProfileRequiresCoreDecision):
		apierror.Write(w, http.StatusConflict, apierror.Error{Code: "current_profile_requires_core_decision", Message: "当前档案需要核心操作层决策", Stage: stage})
	case errors.Is(err, ErrPreconditionRequired):
		apierror.Write(w, http.StatusPreconditionRequired, apierror.Error{Code: "if_match_required", Message: "保存草稿必须携带有效 If-Match", Stage: stage})
	default:
		apierror.WriteError(w, err)
	}
}
