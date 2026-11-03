package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/logger"
)

func decode(t *testing.T, line []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("log line is not JSON: %v (%s)", err, line)
	}
	return m
}

func TestIntoAndFrom(t *testing.T) {
	t.Parallel()

	l := logger.New(&bytes.Buffer{}, slog.LevelInfo)
	ctx := logger.Into(context.Background(), l)
	if got := logger.From(ctx); got != l {
		t.Errorf("From() = %p, want %p", got, l)
	}
}

func TestFromWithoutLoggerReturnsDefault(t *testing.T) { //nolint:paralleltest // reads the global default logger
	if got := logger.From(context.Background()); got != slog.Default() {
		t.Errorf("From() = %p, want slog.Default()", got)
	}
}

func TestErrAttribute(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger.New(&buf, slog.LevelInfo).Info("save failed", logger.Err(errors.New("store: disk full")))

	m := decode(t, buf.Bytes())
	if got, want := m["error"], "store: disk full"; got != want {
		t.Errorf("error = %v, want %v", got, want)
	}
	if got, want := m["msg"], "save failed"; got != want {
		t.Errorf("msg = %v, want %v", got, want)
	}
}

func TestErrAttributeNil(t *testing.T) {
	t.Parallel()

	if got, want := logger.Err(nil).Value.String(), "<nil>"; got != want {
		t.Errorf("Err(nil) = %q, want %q", got, want)
	}
}

func TestPanicAttribute(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger.New(&buf, slog.LevelInfo).Error("recovered", logger.Panic("boom", nil))

	m := decode(t, buf.Bytes())
	group, ok := m["panic"].(map[string]any)
	if !ok {
		t.Fatalf("panic = %v, want a group", m["panic"])
	}
	if got, want := group["value"], "boom"; got != want {
		t.Errorf("panic.value = %v, want %v", got, want)
	}
	if stack, _ := group["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("panic.stack = %q, want a goroutine stack", stack)
	}
}

func TestPanicAttributeKeepsGivenStack(t *testing.T) {
	t.Parallel()

	attr := logger.Panic("boom", []byte("given stack"))
	var stack string
	for _, a := range attr.Value.Group() {
		if a.Key == "stack" {
			stack = a.Value.String()
		}
	}
	if want := "given stack"; stack != want {
		t.Errorf("panic.stack = %q, want %q", stack, want)
	}
}

func TestNewRespectsLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger.New(&buf, slog.LevelWarn).Info("hidden")
	if buf.Len() != 0 {
		t.Errorf("log = %s, want nothing below warn", buf.String())
	}
}

func TestOpenWritesToFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "logs")
	l, closer, err := logger.Open(dir, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Info("started")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, logger.FileName))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if got, want := decode(t, bytes.TrimSpace(data))["msg"], "started"; got != want {
		t.Errorf("msg = %v, want %v", got, want)
	}
}

func TestOpenWithoutFolderUsesStderr(t *testing.T) {
	t.Parallel()

	l, closer, err := logger.Open("", slog.LevelInfo)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if l == nil {
		t.Fatal("Open() logger = nil")
	}
	if err := closer.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
