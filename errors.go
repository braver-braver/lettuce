package lettuce

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrPermission         = errors.New("permission denied")
	ErrInvalid            = errors.New("invalid argument")
	ErrConflict           = errors.New("conflict")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrUnsupported        = errors.New("unsupported")
	ErrUnavailable        = errors.New("storage unavailable")
)

// Error adds operation, provider, and key context while preserving the wrapped
// error for errors.Is and errors.As.
type Error struct {
	Op       string
	Provider string
	Key      string
	Err      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	prefix := e.Op
	if e.Provider != "" {
		if prefix != "" {
			prefix = e.Provider + " " + prefix
		} else {
			prefix = e.Provider
		}
	}
	if e.Key != "" {
		if prefix != "" {
			prefix += " "
		}
		prefix += fmt.Sprintf("%q", e.Key)
	}
	if e.Err == nil {
		if prefix == "" {
			return "storage error"
		}
		return prefix
	}
	if prefix == "" {
		return e.Err.Error()
	}
	return prefix + ": " + e.Err.Error()
}

// Unwrap returns the underlying storage error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
