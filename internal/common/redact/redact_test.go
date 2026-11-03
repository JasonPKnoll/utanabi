package redact_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

const raw = "hunter2-very-secret"

func TestSecretNeverFormatsItsValue(t *testing.T) {
	t.Parallel()

	s := redact.New(raw)
	tests := []struct {
		name string
		got  string
	}{
		{name: "String", got: s.String()},
		{name: "percent v", got: fmt.Sprintf("%v", s)},
		{name: "percent plus v", got: fmt.Sprintf("%+v", s)},
		{name: "percent hash v", got: fmt.Sprintf("%#v", s)},
		{name: "percent s", got: fmt.Sprintf("%s", s)},
		{name: "percent q", got: fmt.Sprintf("%q", s)},
		{name: "percent x", got: fmt.Sprintf("%x", s)},
		{name: "inside a struct", got: fmt.Sprintf("%+v", struct{ Token redact.Secret }{s})},
		{name: "pointer", got: fmt.Sprintf("%v", &s)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(tt.got, raw) {
				t.Errorf("got %q, want no raw value", tt.got)
			}
		})
	}
}

func TestSecretJSONIsRedacted(t *testing.T) {
	t.Parallel()

	got, err := json.Marshal(struct {
		Token redact.Secret `json:"token"`
	}{redact.New(raw)})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"token":"[REDACTED]"}`; string(got) != want {
		t.Errorf("json = %s, want %s", got, want)
	}
}

func TestSecretLogIsRedacted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("login", "token", redact.New(raw))

	if strings.Contains(buf.String(), raw) {
		t.Errorf("log = %s, want no raw value", buf.String())
	}
	if !strings.Contains(buf.String(), "[REDACTED]") {
		t.Errorf("log = %s, want [REDACTED]", buf.String())
	}
}

func TestRevealReturnsTheValue(t *testing.T) {
	t.Parallel()

	if got := redact.New(raw).Reveal(); got != raw {
		t.Errorf("Reveal() = %q, want %q", got, raw)
	}
}
