// Package apperrors keeps stable presentation codes separate from wrapped causes.
package apperrors

import (
	"context"
	"errors"
	"io/fs"
)

type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string           { return e.Cause.Error() }
func (e *Error) Unwrap() error           { return e.Cause }
func New(code string, cause error) error { return &Error{Code: code, Cause: cause} }

type clientError struct {
	code  string
	cause error
}

func (e *clientError) Error() string { return "nte:" + e.code }
func (e *clientError) Unwrap() error { return e.cause }

// ForClient hides paths and technical prose from Wails' string error transport.
// Callers retain the original error for diagnostics before crossing this boundary.
func ForClient(err error) error {
	if err == nil {
		return nil
	}
	var classified *Error
	code := "operation_failed"
	switch {
	case errors.As(err, &classified):
		code = classified.Code
	case errors.Is(err, context.Canceled):
		code = "cancelled"
	case errors.Is(err, fs.ErrNotExist):
		code = "data_missing"
	case errors.Is(err, fs.ErrPermission):
		code = "data_unavailable"
	}
	return &clientError{code: code, cause: err}
}
