// Package fault holds the project's own error sentinels and the invalid input error.
// Adapters translate their dependency errors into these, and internal/app maps them to frontend error codes.
package fault

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound = errors.New("fault: not found")
	ErrConflict = errors.New("fault: conflict")
	ErrCanceled = errors.New("fault: canceled")
	ErrTimeout  = errors.New("fault: timeout")
)

// InvalidError reports input that failed validation, and names the field at fault.
type InvalidError struct {
	Field  string
	Reason string
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("fault: invalid input (%s %s)", e.Field, e.Reason)
}

// Invalid returns an *InvalidError for the given field.
func Invalid(field, reason string) error {
	return &InvalidError{Field: field, Reason: reason}
}
