package capabilities

import (
	"net/http"

	"singbox-web/internal/httpapi"
)

// NewHandler returns the read-only capability endpoint handler.
func NewHandler(response Response) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpapi.MethodNotAllowed(w, http.MethodGet)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	})
}
