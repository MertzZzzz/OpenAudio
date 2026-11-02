package domain

import (
	"errors"
	"fmt"
)

// Kind classifies domain errors so transports can map them (e.g. to HTTP status codes)
// without knowing about individual error codes.
type Kind int

const (
	KindInternal Kind = iota
	KindValidation
	KindNotFound
	KindConflict
	KindUnauthorized
	KindForbidden
	KindRateLimited
)

// FieldError describes a single invalid input field.
type FieldError struct {
	Field   string `json:"path"`
	Message string `json:"message"`
}

// Error is the single error type returned by use cases.
type Error struct {
	Kind    Kind
	Code    string // stable, machine-readable: "email_taken", "invalid_credentials", ...
	Message string // human-readable, safe to show to clients
	Fields  []FieldError
	Err     error // wrapped cause, never exposed to clients
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Is lets errors.Is match on Code, e.g. errors.Is(err, domain.ErrNotFound).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

func newErr(kind Kind, code, msg string) *Error { return &Error{Kind: kind, Code: code, Message: msg} }

// Sentinel errors shared by repositories and use cases.
var (
	ErrNotFound = newErr(KindNotFound, "not_found", "resource not found")
	ErrConflict = newErr(KindConflict, "conflict", "resource already exists")
)

func NotFound(code, msg string) *Error     { return newErr(KindNotFound, code, msg) }
func Conflict(code, msg string) *Error     { return newErr(KindConflict, code, msg) }
func Unauthorized(code, msg string) *Error { return newErr(KindUnauthorized, code, msg) }
func Forbidden(code, msg string) *Error    { return newErr(KindForbidden, code, msg) }
func RateLimited() *Error {
	return newErr(KindRateLimited, "rate_limited", "too many requests, slow down")
}

// Validation builds a validation error from field errors.
func Validation(fields ...FieldError) *Error {
	e := newErr(KindValidation, "validation_failed", "request validation failed")
	e.Fields = fields
	return e
}

// Internal wraps an unexpected error.
func Internal(err error) *Error {
	e := newErr(KindInternal, "internal", "internal server error")
	e.Err = err
	return e
}

// KindOf returns the Kind of err, KindInternal for foreign errors.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}
