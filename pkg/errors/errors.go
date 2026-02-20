// Package errors provides the unified error type and factory functions for the
// KeyIP-Intelligence platform.  Every layer of the application (domain, application,
// infrastructure, interfaces) uses AppError as the single carrier for structured
// error information, enabling consistent HTTP responses, logging, and monitoring.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Build-tag / compile-time stack-capture control
//
// By default stack traces are captured on every New/Wrap call.  In
// performance-sensitive production deployments set the build tag
// "nostack" to compile out the runtime.Callers call entirely:
//
//   go build -tags nostack ./...
// ─────────────────────────────────────────────────────────────────────────────

// stackDepth is the maximum number of frames captured per error.
const stackDepth = 32

// captureStack returns a formatted call-stack string starting two frames above
// the caller (skipping captureStack itself and New/Wrap).  When compiled with
// the "nostack" build tag this function is replaced by a no-op stub in
// stack_disabled.go so there is zero runtime overhead.
func captureStack(skip int) string {
	pcs := make([]uintptr, stackDepth)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder
	for {
		f, more := frames.Next()
		// Trim standard-library noise to keep traces readable.
		if !strings.Contains(f.File, "runtime/") {
			fmt.Fprintf(&sb, "\n\t%s:%d %s", f.File, f.Line, f.Function)
		}
		if !more {
			break
		}
	}
	return sb.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// AppError — the canonical platform error type
// ─────────────────────────────────────────────────────────────────────────────

// AppError is the single structured error type used throughout KeyIP-Intelligence.
// It satisfies the standard error interface and supports Go 1.13+ error wrapping
// so that errors.Is / errors.As / errors.Unwrap work transparently across all
// layers of the application.
//
// Usage:
//
//	return errors.New(errors.CodePatentNotFound, "patent CN202310001234A not found")
//	return errors.Wrap(repoErr, errors.CodeDBConnectionError, "failed to query patent")
//	return errors.NotFound("molecule with InChIKey XXXXXXXXXXXXXXXX not found").
//	           WithDetail("searched in postgres and milvus")
type AppError struct {
	// Code is the typed error code that uniquely identifies the failure category.
	Code ErrorCode

	// Message is the primary human-readable description of the error, suitable
	// for inclusion in API responses returned to callers.
	Message string

	// Detail carries supplementary context (query parameters, entity IDs, etc.)
	// that aids debugging without leaking sensitive internals to end users.
	Detail string

	// Cause is the underlying error that triggered this AppError, enabling
	// errors.Is / errors.As traversal of the full error chain.
	Cause error

	// Stack contains the formatted call-stack captured at the point of error
	// creation.  It is populated by New and Wrap but omitted when the "nostack"
	// build tag is set.  Stack is intentionally not included in Error() output
	// to keep API error messages clean; callers that need it can inspect the
	// field directly (e.g., structured logger middleware).
	Stack string
}

// ─────────────────────────────────────────────────────────────────────────────
// error interface implementation
// ─────────────────────────────────────────────────────────────────────────────

// Error implements the standard error interface.
// Format: "[<code_name>(<code_int>)] <message>: <detail>"
// The detail segment is omitted when Detail is empty.
func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s(%d)] %s: %s", e.Code.String(), int(e.Code), e.Message, e.Detail)
	}
	return fmt.Sprintf("[%s(%d)] %s", e.Code.String(), int(e.Code), e.Message)
}

// Unwrap returns the underlying cause error, enabling errors.Is and errors.As
// to traverse the full error chain without any additional boilerplate at call sites.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// ─────────────────────────────────────────────────────────────────────────────
// Fluent builder methods
// ─────────────────────────────────────────────────────────────────────────────

// WithDetail returns a shallow copy of the receiver with Detail set to the
// supplied string.  It is safe to call on a nil pointer (returns nil).
// Example:
//
//	return errors.NotFound("patent not found").WithDetail("id=" + id)
func (e *AppError) WithDetail(detail string) *AppError {
	if e == nil {
		return nil
	}
	clone := *e
	clone.Detail = detail
	return &clone
}

// WithCause returns a shallow copy of the receiver with Cause set to err.
// Use this when you want to attach a lower-level error to an already-constructed
// AppError without going through Wrap.
func (e *AppError) WithCause(err error) *AppError {
	if e == nil {
		return nil
	}
	clone := *e
	clone.Cause = err
	return &clone
}

// ─────────────────────────────────────────────────────────────────────────────
// Primary factory functions
// ─────────────────────────────────────────────────────────────────────────────

// New constructs a fresh AppError with the given code and message.
// A call-stack snapshot is captured automatically (unless compiled with -tags nostack).
//
// New is the preferred factory for errors that originate in the current layer
// without an underlying cause from a lower layer.
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Stack:   captureStack(1),
	}
}

// Wrap constructs an AppError that wraps an existing error.
// If err is nil, Wrap returns nil so it can be used inline:
//
//	return errors.Wrap(repo.FindByID(ctx, id), errors.CodeDBConnectionError, "query failed")
//
// When err is already an *AppError and code is CodeUnknown the original code is
// preserved, preventing loss of the original domain classification during
// cross-layer propagation.
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	// Preserve original code when the caller is just adding context.
	if code == CodeUnknown {
		var ae *AppError
		if errors.As(err, &ae) {
			code = ae.Code
		}
	}
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   err,
		Stack:   captureStack(1),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Error-chain inspection helpers
// ─────────────────────────────────────────────────────────────────────────────

// IsCode reports whether any error in err's chain is an *AppError with the
// given code.  It is the idiomatic way to check domain-specific failure modes:
//
//	if errors.IsCode(err, errors.CodePatentNotFound) { ... }
func IsCode(err error, code ErrorCode) bool {
	var ae *AppError
	for err != nil {
		if errors.As(err, &ae) && ae.Code == code {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// IsNotFound reports whether any error in err's chain is an *AppError with
// CodeNotFound, CodePatentNotFound, CodeMoleculeNotFound, or CodePortfolioNotFound.
func IsNotFound(err error) bool {
	var ae *AppError
	for err != nil {
		if errors.As(err, &ae) {
			switch ae.Code {
			case CodeNotFound, CodePatentNotFound, CodeMoleculeNotFound, CodePortfolioNotFound:
				return true
			}
		}
		err = errors.Unwrap(err)
	}
	return false
}

// GetCode extracts the ErrorCode from the first *AppError found in err's chain.
// If no *AppError is present, CodeUnknown is returned.
//
// This is useful in middleware / logging layers that need a single code to emit
// as a metric label without coupling to specific domain errors.
func GetCode(err error) ErrorCode {
	if err == nil {
		return CodeOK
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return CodeUnknown
}

// ─────────────────────────────────────────────────────────────────────────────
// Convenience factory functions for the most common error conditions
// ─────────────────────────────────────────────────────────────────────────────
// Each function mirrors the pattern used in well-known Go HTTP frameworks so
// that call sites read naturally:
//
//   return errors.NotFound("patent CN202310001234A")
//   return errors.InvalidParam("SMILES must not be empty")

// NotFound constructs a CodeNotFound AppError.
// Prefer CodePatentNotFound / CodeMoleculeNotFound / CodePortfolioNotFound for
// domain-specific variants; this generic form is appropriate in generic
// repository or router layers.
func NotFound(message string) *AppError {
	return &AppError{
		Code:    CodeNotFound,
		Message: message,
		Stack:   captureStack(1),
	}
}

// InvalidParam constructs a CodeInvalidParam AppError.
func InvalidParam(message string) *AppError {
	return &AppError{
		Code:    CodeInvalidParam,
		Message: message,
		Stack:   captureStack(1),
	}
}

// InvalidState constructs a CodeConflict AppError, used for domain state violations.
func InvalidState(message string) *AppError {
	return &AppError{
		Code:    CodeConflict,
		Message: message,
		Stack:   captureStack(1),
	}
}

// Unauthorized constructs a CodeUnauthorized AppError.
func Unauthorized(message string) *AppError {
	return &AppError{
		Code:    CodeUnauthorized,
		Message: message,
		Stack:   captureStack(1),
	}
}

// Forbidden constructs a CodeForbidden AppError.
func Forbidden(message string) *AppError {
	return &AppError{
		Code:    CodeForbidden,
		Message: message,
		Stack:   captureStack(1),
	}
}

// Internal constructs a CodeInternal AppError.
// Use this for unexpected server-side failures where no more specific code
// applies.  Always log the underlying cause before or after calling Internal.
func Internal(message string) *AppError {
	return &AppError{
		Code:    CodeInternal,
		Message: message,
		Stack:   captureStack(1),
	}
}

// Conflict constructs a CodeConflict AppError.
func Conflict(message string) *AppError {
	return &AppError{
		Code:    CodeConflict,
		Message: message,
		Stack:   captureStack(1),
	}
}

// RateLimit constructs a CodeRateLimit AppError.
func RateLimit(message string) *AppError {
	return &AppError{
		Code:    CodeRateLimit,
		Message: message,
		Stack:   captureStack(1),
	}
}

