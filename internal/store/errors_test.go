package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

func TestTranslate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   error
		want error
	}{
		{name: "no rows", in: sql.ErrNoRows, want: fault.ErrNotFound},
		{name: "wrapped no rows", in: fmt.Errorf("scan: %w", sql.ErrNoRows), want: fault.ErrNotFound},
		{name: "canceled", in: context.Canceled, want: fault.ErrCanceled},
		{name: "deadline", in: context.DeadlineExceeded, want: fault.ErrTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := store.Translate(tt.in); !errors.Is(got, tt.want) {
				t.Errorf("Translate(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestTranslateNil(t *testing.T) {
	t.Parallel()

	if got := store.Translate(nil); got != nil {
		t.Errorf("Translate(nil) = %v, want nil", got)
	}
}

func TestTranslateUnknownHidesDriverType(t *testing.T) {
	t.Parallel()

	driverErr := errors.New("near \"SELEC\": syntax error")
	got := store.Translate(driverErr)
	if errors.Is(got, driverErr) {
		t.Errorf("Translate() wraps the driver error with %%w, want it hidden")
	}
	if want := `store: query failed (near "SELEC": syntax error)`; got.Error() != want {
		t.Errorf("Translate() = %q, want %q", got, want)
	}
}
