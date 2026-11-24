// Package apperr holds the error kinds the library services return and the
// HTTP layer maps to status codes.
package apperr

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"generic-arr-project/internal/formspec"
)

// ErrNotFound means the requested item does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidCredentials is returned for a wrong password or session.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ValidationError is a problem with caller-supplied input.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Invalid returns a ValidationError for field.
func Invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// PrefixField namespaces a validation error's field ("downloads.complete_dir").
// Other errors are returned unchanged.
func PrefixField(prefix string, err error) error {
	if ve, ok := err.(*ValidationError); ok {
		return &ValidationError{Field: prefix + ve.Field, Message: ve.Message}
	}
	return err
}

// ConflictError means the request clashes with existing state.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// Conflict returns a ConflictError.
func Conflict(format string, args ...any) error {
	return &ConflictError{Message: fmt.Sprintf(format, args...)}
}

// RateLimitedError says how long a client must wait before retrying.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("too many failed logins; try again in %s", e.RetryAfter.Round(time.Second))
}

// NotFound maps sql.ErrNoRows to ErrNotFound with context.
func NotFound(err error, what string, id int64) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s %d: %w", what, id, ErrNotFound)
	}
	return fmt.Errorf("load %s %d: %w", what, id, err)
}

// IsUniqueViolation reports whether err is a SQLite unique constraint failure.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// FromFieldError turns a form field error into a ValidationError naming
// the field. Other errors are returned unchanged.
func FromFieldError(err error) error {
	var fe *formspec.FieldError
	if errors.As(err, &fe) {
		return Invalid(fe.Field, "%s", fe.Message)
	}
	return err
}
