package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/logger"
)

func captureLog(t *testing.T) (context.Context, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return logger.Into(context.Background(), logger.New(&buf, nil)), &buf
}

func TestFailMapsErrors(t *testing.T) {
	t.Parallel()

	const private = "/Users/someone/private.db"
	tests := []struct {
		name      string
		err       error
		wantCode  string
		wantField string
		wantLevel string
	}{
		{name: "invalid input", err: fault.Invalid("title", "must not be empty"),
			wantCode: CodeInvalidInput, wantField: "title", wantLevel: "INFO"},
		{name: "not found", err: fmt.Errorf("songs: load failed (%s); %w", private, fault.ErrNotFound),
			wantCode: CodeNotFound, wantLevel: "INFO"},
		{name: "conflict", err: fmt.Errorf("songs: save failed; %w", fault.ErrConflict),
			wantCode: CodeConflict, wantLevel: "INFO"},
		{name: "canceled sentinel", err: fault.ErrCanceled, wantCode: CodeCanceled, wantLevel: "INFO"},
		{name: "context canceled", err: context.Canceled, wantCode: CodeCanceled, wantLevel: "INFO"},
		{name: "timeout sentinel", err: fault.ErrTimeout, wantCode: CodeTimeout, wantLevel: "WARN"},
		{name: "context deadline", err: context.DeadlineExceeded, wantCode: CodeTimeout, wantLevel: "WARN"},
		{name: "unknown", err: fmt.Errorf("store: query failed (%s)", private), wantCode: CodeInternal, wantLevel: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, buf := captureLog(t)

			got := Fail(ctx, tt.err)

			var fe *Error
			if !errors.As(got, &fe) {
				t.Fatalf("Fail() = %T, want *Error", got)
			}
			if fe.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", fe.Code, tt.wantCode)
			}
			if fe.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", fe.Field, tt.wantField)
			}
			if strings.Contains(got.Error(), private) {
				t.Errorf("Error() = %q, want no internal detail", got.Error())
			}

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if len(lines) != 1 {
				t.Fatalf("logged %d lines, want exactly 1", len(lines))
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
				t.Fatalf("log line is not JSON: %v", err)
			}
			if entry["level"] != tt.wantLevel {
				t.Errorf("level = %v, want %v", entry["level"], tt.wantLevel)
			}
			if entry["code"] != tt.wantCode {
				t.Errorf("logged code = %v, want %v", entry["code"], tt.wantCode)
			}
		})
	}
}

func TestFailNil(t *testing.T) {
	t.Parallel()

	ctx, buf := captureLog(t)
	if got := Fail(ctx, nil); got != nil {
		t.Errorf("Fail(nil) = %v, want nil", got)
	}
	if buf.Len() != 0 {
		t.Errorf("log = %s, want nothing", buf.String())
	}
}

func TestFailKeepsFrontendErrorAndDoesNotLogTwice(t *testing.T) {
	t.Parallel()

	ctx, buf := captureLog(t)
	first := Fail(ctx, fault.ErrNotFound)
	buf.Reset()

	if got := Fail(ctx, first); !errors.Is(got, first) {
		t.Errorf("Fail(*Error) = %v, want the same error", got)
	}
	if buf.Len() != 0 {
		t.Errorf("log = %s, want nothing on the second call", buf.String())
	}
}

func TestMarshalErrorSendsCodeAndMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	data := marshalError(logger.New(&buf, nil))(&Error{Code: CodeNotFound, Message: "Not found."})

	if want := `{"code":"not_found","message":"Not found."}`; string(data) != want {
		t.Errorf("marshal = %s, want %s", data, want)
	}
	if buf.Len() != 0 {
		t.Errorf("log = %s, want nothing for a mapped error", buf.String())
	}
}

func TestMarshalErrorMasksUnmappedError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	data := marshalError(logger.New(&buf, nil))(errors.New("store: query failed (secret detail)"))

	var got Error
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Code != CodeInternal || strings.Contains(string(data), "secret detail") {
		t.Errorf("marshal = %s, want an internal error without detail", data)
	}
	if !strings.Contains(buf.String(), "without app.Fail") {
		t.Errorf("log = %s, want the unmapped error logged", buf.String())
	}
}
