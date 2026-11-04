// Package redact provides the Secret type, which never prints its value.
package redact

import (
	"fmt"
	"log/slog"
)

const placeholder = "[REDACTED]"

// Secret holds a sensitive value. Formatting, logging, and JSON all show [REDACTED].
// Call Reveal only where the value leaves the process.
type Secret struct {
	value string
}

// New wraps a sensitive value.
func New(value string) Secret {
	return Secret{value: value}
}

// Reveal returns the raw value.
func (s Secret) Reveal() string {
	return s.value
}

// IsZero reports whether the secret is empty.
func (s Secret) IsZero() bool {
	return s.value == ""
}

func (Secret) String() string {
	return placeholder
}

func (Secret) GoString() string {
	return placeholder
}

// Format covers every verb, including %v with flags and %x, which would otherwise bypass String.
func (Secret) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(placeholder))
}

func (Secret) LogValue() slog.Value {
	return slog.StringValue(placeholder)
}

func (Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + placeholder + `"`), nil
}

func (Secret) MarshalText() ([]byte, error) {
	return []byte(placeholder), nil
}
