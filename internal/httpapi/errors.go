package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"generic-arr-project/internal/library/apperr"
)

const setupRequiredMessage = "setup required: complete onboarding at /api/v1/setup"

var (
	errBadRequest    = errors.New("bad request")
	errSetupRequired = errors.New("setup required")
)

// errorBody is the error response written outside the generated handlers
// (guards, validation, error mapping); it matches the spec's Error schema.
type errorBody struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr maps service errors to HTTP statuses. Unexpected errors are
// logged and reported as 500 with their message (single-user admin API).
func (h *handlers) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve *apperr.ValidationError
	var ce *apperr.ConflictError
	var rl *apperr.RateLimitedError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: ve.Message, Field: ve.Field})
	case errors.As(err, &ce):
		writeJSON(w, http.StatusConflict, errorBody{Error: ce.Message})
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		writeJSON(w, http.StatusTooManyRequests, errorBody{Error: rl.Error()})
	case errors.Is(err, apperr.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "wrong password"})
	case errors.Is(err, apperr.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: err.Error()})
	case errors.Is(err, errSetupRequired):
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: setupRequiredMessage})
	case errors.Is(err, errBadRequest):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
	default:
		if r.Context().Err() != nil {
			// The client went away (e.g. the page changed); nothing failed.
			h.log.Debug().Str("method", r.Method).Str("path", r.URL.Path).Err(err).Msg("request canceled")
		} else {
			h.log.Error().Str("method", r.Method).Str("path", r.URL.Path).Err(err).Msg("request failed")
		}
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
	}
}

// badRequest reports a request the generated code could not parse.
func (h *handlers) badRequest(w http.ResponseWriter, r *http.Request, err error) {
	h.writeErr(w, r, fmt.Errorf("%w: %v", errBadRequest, err))
}
