package spotify

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

func TestMorphWritesNamesInRomaji(t *testing.T) {
	t.Parallel()

	m := NewMorph(morph.New())
	tests := []struct{ in, want string }{
		{in: "私生活", want: "shiseikatsu"},
		{in: "東京事変", want: "toukyou jihen"},
		{in: "夜に駆ける", want: "yoru ni kakeru"},
		{in: "死ぬのがいいわ", want: "shinu no ga ii wa"},
		{in: "Lost Umbrella", want: "Lost Umbrella"},
	}
	for _, tt := range tests {
		got, err := m.Romaji(t.Context(), tt.in)
		if err != nil {
			t.Fatalf("Romaji(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("Romaji(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
