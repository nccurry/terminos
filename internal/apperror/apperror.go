// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apperror defines typed errors and stable process exit codes.
package apperror

import (
	"context"
	"errors"
	"fmt"
)

// Stable exit codes used by terminos.
const (
	ExitSuccess         = 0
	ExitInternal        = 1
	ExitUsage           = 2
	ExitValidation      = 3
	ExitCapturedCommand = 4
	ExitTimeout         = 5
	ExitFilesystem      = 6
	ExitRedactionPolicy = 7
	ExitInterrupted     = 130
)

// Code identifies the class of a user-visible error.
type Code int

// Error code values.
const (
	CodeInternal Code = iota + 1
	CodeUsage
	CodeValidation
	CodeCapturedCommand
	CodeTimeout
	CodeFilesystem
	CodeRedactionPolicy
)

// Error is an error with a stable exit-code class.
type Error struct {
	Code    Code
	Message string
	Err     error
}

// New returns an application error.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf returns an application error with a formatted message.
func Newf(code Code, format string, args ...any) *Error {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap returns an application error that wraps a lower-level cause.
func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// Wrapf returns an application error with a formatted message and wrapped cause.
func Wrapf(code Code, err error, format string, args ...any) *Error {
	return Wrap(code, fmt.Sprintf(format, args...), err)
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ExitCode maps an error to a stable process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ExitTimeout
	}

	var appErr *Error
	if !errors.As(err, &appErr) {
		return ExitInternal
	}

	switch appErr.Code {
	case CodeUsage:
		return ExitUsage
	case CodeValidation:
		return ExitValidation
	case CodeCapturedCommand:
		return ExitCapturedCommand
	case CodeTimeout:
		return ExitTimeout
	case CodeFilesystem:
		return ExitFilesystem
	case CodeRedactionPolicy:
		return ExitRedactionPolicy
	default:
		return ExitInternal
	}
}
