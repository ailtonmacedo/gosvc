package clierror

import (
	"errors"
	"fmt"
)

type Code int

const (
	CodeGeneral Code = 1 + iota
	CodeInvalidConfig
	CodeFileConflict
	CodeMissingDependency
	CodeGenerationFailure
	CodeInvalidProject
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" && e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "gosvc error"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(code Code, message string) error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, cause error) error {
	if cause == nil {
		return &Error{Code: code, Message: message}
	}
	return &Error{Code: code, Message: message, Cause: cause}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var target *Error
	if errors.As(err, &target) {
		return int(target.Code)
	}
	return int(CodeGeneral)
}

func DebugString(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%+v", err)
}
