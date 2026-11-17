package dictionary_test

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
)

func TestGrammarIsBuiltIn(t *testing.T) {
	t.Parallel()

	list, err := dictionary.Grammar()
	if err != nil {
		t.Fatalf("Grammar: %v", err)
	}
	if len(list) < 50 {
		t.Fatalf("Grammar has %d points, want the whole list from grammar.json", len(list))
	}
	if first := list[0]; first.ID != "ga-subject" || first.Kind != dictionary.KindParticle || first.Title == "" {
		t.Errorf("first grammar point = %+v, want ga-subject, a particle with a title", first)
	}

	list[0].Title = "changed"
	again, err := dictionary.Grammar()
	if err != nil {
		t.Fatalf("Grammar: %v", err)
	}
	if again[0].Title == "changed" {
		t.Error("changing the returned list changed the built-in one")
	}
}
