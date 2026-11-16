// Package logger builds the JSON slog logger and carries it in the context.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
)

// FileName is the log file inside the log folder.
const FileName = "utanabi.log"

type ctxKey struct{}

// New returns a JSON logger that writes to w.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// Open returns a logger that appends to FileName in dir, or writes to stderr when dir is empty.
// Close the returned io.Closer at shutdown.
func Open(dir string, level slog.Leveler) (*slog.Logger, io.Closer, error) {
	if dir == "" {
		return New(os.Stderr, level), io.NopCloser(nil), nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("logger: create log folder failed (%v)", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, FileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("logger: open log file failed (%v)", err)
	}
	return New(f, level), f, nil
}

// Into returns a copy of ctx that carries l.
func Into(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// From returns the logger in ctx, or slog.Default() when there is none.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// Err returns the attribute for an error. Use it instead of a raw "error" key.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "<nil>")
	}
	return slog.String("error", err.Error())
}

// Panic returns the attribute for a recovered panic value and its stack.
// Pass a nil stack inside a deferred recover to capture the current goroutine's stack.
func Panic(v any, stack []byte) slog.Attr {
	if stack == nil {
		stack = debug.Stack()
	}
	return slog.Group("panic",
		slog.String("value", fmt.Sprint(v)),
		slog.String("stack", string(stack)),
	)
}
