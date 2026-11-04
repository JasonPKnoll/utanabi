package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/logger"
)

// The stable error codes the frontend sees. The frontend switches on these, never on messages.
const (
	CodeInvalidInput = "invalid_input"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeCanceled     = "canceled"
	CodeTimeout      = "timeout"
	CodeInternal     = "internal"
)

const internalMessage = "Something went wrong. Please try again."

// Error is the error the frontend sees: a stable code and a message that is safe to show.
// Wails sends Error() to the frontend as the message, so it must never hold internal detail.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// Fail turns a domain error into the frontend error and logs the failure once.
// Every bound service method returns its error through Fail. It returns nil for a nil error.
func Fail(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var fe *Error
	if errors.As(err, &fe) {
		return fe
	}

	fe, level := toFrontend(err)
	logger.From(ctx).LogAttrs(ctx, level, "call failed", slog.String("code", fe.Code), logger.Err(err))
	return fe
}

func toFrontend(err error) (*Error, slog.Level) {
	var invalid *fault.InvalidError
	switch {
	case errors.As(err, &invalid):
		return &Error{Code: CodeInvalidInput, Message: invalid.Field + " " + invalid.Reason, Field: invalid.Field},
			slog.LevelInfo
	case errors.Is(err, fault.ErrNotFound):
		return &Error{Code: CodeNotFound, Message: "Not found."}, slog.LevelInfo
	case errors.Is(err, fault.ErrConflict):
		return &Error{Code: CodeConflict, Message: "That already exists."}, slog.LevelInfo
	case errors.Is(err, fault.ErrCanceled), errors.Is(err, context.Canceled):
		return &Error{Code: CodeCanceled, Message: "The request was canceled."}, slog.LevelInfo
	case errors.Is(err, fault.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return &Error{Code: CodeTimeout, Message: "That took too long. Please try again."}, slog.LevelWarn
	default:
		return &Error{Code: CodeInternal, Message: internalMessage}, slog.LevelError
	}
}

// marshalError is the Wails MarshalError hook. It sends an *Error as JSON in the error's cause.
// Anything else skipped Fail, so it is logged as a bug and sent as a plain internal error.
func marshalError(log *slog.Logger) func(error) []byte {
	return func(err error) []byte {
		var fe *Error
		if !errors.As(err, &fe) {
			log.LogAttrs(context.Background(), slog.LevelError, "error reached the frontend without app.Fail",
				logger.Err(err))
			fe = &Error{Code: CodeInternal, Message: internalMessage}
		}
		data, jsonErr := json.Marshal(fe)
		if jsonErr != nil {
			return nil
		}
		return data
	}
}
