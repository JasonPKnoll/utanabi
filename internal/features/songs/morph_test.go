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
		{Surface: "星", Reading: "ほし", POS: morph.Noun, Detail: "一般", BaseForm: "星"},
		{Surface: "が", Reading: "が", POS: morph.Particle, Detail: "格助詞", Detail2: "一般", BaseForm: "が"},
		{Surface: "光る", Reading: "ひかる", POS: morph.Verb, Detail: "自立", BaseForm: "光る", ConjType: "五段・ラ行",
			ConjForm: "基本形"},
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
