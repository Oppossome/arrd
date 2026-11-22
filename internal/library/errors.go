// Package library holds arrd's application logic: settings, indexers, the
// movie library, searching, grabbing and importing.
package library

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound means the requested item does not exist.
var ErrNotFound = errors.New("not found")

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

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// ConflictError means the request clashes with existing state.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

func conflict(format string, args ...any) error {
	return &ConflictError{Message: fmt.Sprintf(format, args...)}
}

// notFound maps sql.ErrNoRows to ErrNotFound with context.
func notFound(err error, what string, id int64) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s %d: %w", what, id, ErrNotFound)
	}
	return fmt.Errorf("load %s %d: %w", what, id, err)
}
