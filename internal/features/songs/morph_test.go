package songs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

func TestMorphAnalyzesALine(t *testing.T) {
	t.Parallel()

	got, err := NewMorph(morph.New()).Analyze(t.Context(), "星が光る")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := []Token{
		{Surface: "星", Reading: "ほし", POS: morph.Noun},
		{Surface: "が", Reading: "が", POS: morph.Particle},
		{Surface: "光る", Reading: "ひかる", POS: morph.Verb},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMorphStopsWhenTheCallIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewMorph(morph.New()).Analyze(ctx, "星"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
