package generator

import (
	"errors"
	"fmt"
)

type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}

func conflictf(format string, arguments ...any) error {
	return &ConflictError{Message: fmt.Sprintf(format, arguments...)}
}

func IsConflict(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}
