package httpapi

import (
	"encoding/json"
	"net/http"

	"singbox-web/internal/validation"
)

type ErrorBody struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code            string             `json:"code"`
	Message         string             `json:"message"`
	FieldIssues     []validation.Issue `json:"fieldIssues,omitempty"`
	CurrentRevision *uint64            `json:"currentRevision,omitempty"`
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeAPIError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, ErrorBody{Error: APIError{Code: code, Message: message}})
}
