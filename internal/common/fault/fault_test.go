package fault_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

func TestInvalidMatchesWithErrorsAs(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("songs: save failed; %w", fault.Invalid("title", "must not be empty"))

	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("errors.As(%v) = false, want true", err)
	}
	if got, want := invalid.Field, "title"; got != want {
		t.Errorf("Field = %q, want %q", got, want)
	}
}

func TestInvalidErrorString(t *testing.T) {
	t.Parallel()

	got := fault.Invalid("title", "must not be empty").Error()
	want := "fault: invalid input (title must not be empty)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
