package jtext_test

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

func TestToHiragana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "カタカナ", want: "かたかな"},
		{in: "ラーメン", want: "らーめん"},
		{in: "ヴァイオリン", want: "ゔぁいおりん"},
		{in: "マッチャ", want: "まっちゃ"},
		{in: "ヵヶ", want: "ゕゖ"},
		{in: "ヽヾ", want: "ゝゞ"},
		{in: "ひらがな", want: "ひらがな"},
		{in: "光るLOVE、", want: "光るLOVE、"},
		{in: "ヷ", want: "ヷ"},
		{in: "", want: ""},
	}
	for _, tt := range tests {
		if got := jtext.ToHiragana(tt.in); got != tt.want {
			t.Errorf("ToHiragana(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToHiraganaGivesAStorableReading(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"ヴ", "ァィゥェォ", "ッャュョヮ", "ヵヶ", "ー", "ヽヾ"} {
		if got := jtext.ToHiragana(in); !jtext.IsHiragana(got) {
			t.Errorf("ToHiragana(%q) = %q, which IsHiragana refuses", in, got)
		}
	}
}

func TestIsKana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "ひかる", want: true},
		{in: "ラーメン", want: true},
		{in: "ゝ", want: true},
		{in: "", want: false},
		{in: "光る", want: false},
		{in: "時々", want: false},
		{in: "love", want: false},
		{in: "、", want: false},
	}
	for _, tt := range tests {
		if got := jtext.IsKana(tt.in); got != tt.want {
			t.Errorf("IsKana(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
