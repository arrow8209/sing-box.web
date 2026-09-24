package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

var (
	ErrInvalidMode  = errors.New("invalid_runtime_mode")
	ErrModeDisabled = errors.New("runtime_mode_unavailable")
	ErrModeChange   = errors.New("runtime_mode_change_failed")
)

type ModeSelection struct {
	Mode string `json:"mode"`
}

type ModeSnapshot struct {
	Mode           string   `json:"mode"`
	AvailableModes []string `json:"availableModes"`
}

func validMode(mode string) bool {
	return mode == "rule" || mode == "global" || mode == "direct"
}

// Mode reads the core's current choice and supported modes, never the draft's
// default_mode. Only the mode fields are exposed to the browser.
func (s *Service) Mode(ctx context.Context) (ModeSnapshot, error) {
	response, err := s.request(ctx, http.MethodGet, "/configs", nil)
	if err != nil {
		return ModeSnapshot{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ModeSnapshot{}, fmt.Errorf("%w: status %d", ErrControllerUnavailable, response.StatusCode)
	}
	const maximumModeBody = 64 << 10
	payload, err := io.ReadAll(io.LimitReader(response.Body, maximumModeBody+1))
	if err != nil || len(payload) > maximumModeBody {
		return ModeSnapshot{}, fmt.Errorf("%w: unreadable mode response", ErrControllerUnavailable)
	}
	var config struct {
		Mode     string   `json:"mode"`
		ModeList []string `json:"mode-list"`
	}
	if err := json.Unmarshal(payload, &config); err != nil || strings.TrimSpace(config.Mode) == "" {
		return ModeSnapshot{}, fmt.Errorf("%w: invalid mode response", ErrControllerUnavailable)
	}
	snapshot := ModeSnapshot{Mode: strings.ToLower(config.Mode), AvailableModes: []string{}}
	for _, mode := range config.ModeList {
		mode = strings.ToLower(mode)
		if validMode(mode) && !slices.Contains(snapshot.AvailableModes, mode) {
			snapshot.AvailableModes = append(snapshot.AvailableModes, mode)
		}
	}
	return snapshot, nil
}

func (s *Service) SetMode(ctx context.Context, input ModeSelection) (ModeSnapshot, error) {
	if !validMode(input.Mode) {
		return ModeSnapshot{}, ErrInvalidMode
	}
	current, err := s.Mode(ctx)
	if err != nil {
		return ModeSnapshot{}, err
	}
	if !slices.Contains(current.AvailableModes, input.Mode) {
		return ModeSnapshot{}, ErrModeDisabled
	}
	if current.Mode == input.Mode {
		return current, nil
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return ModeSnapshot{}, ErrInvalidMode
	}
	response, err := s.request(ctx, http.MethodPatch, "/configs", bytes.NewReader(payload))
	if err != nil {
		return ModeSnapshot{}, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
	case http.StatusBadRequest, http.StatusConflict:
		return ModeSnapshot{}, ErrModeChange
	default:
		return ModeSnapshot{}, fmt.Errorf("%w: status %d", ErrControllerUnavailable, response.StatusCode)
	}
	// A successful PATCH can still be ignored by sing-box. Confirm the actual
	// mode before reporting success, including its case-insensitive mode names.
	confirmed, err := s.Mode(ctx)
	if err != nil {
		return ModeSnapshot{}, err
	}
	if confirmed.Mode != input.Mode {
		return ModeSnapshot{}, ErrModeChange
	}
	return confirmed, nil
}
