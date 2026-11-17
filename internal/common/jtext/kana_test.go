package jtext_test

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

func TestToHiragana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "hiragana is unchanged", in: "ひかる", want: "ひかる"},
		{name: "katakana becomes hiragana", in: "カラ", want: "から"},
		{name: "small kana and the first and last letters", in: "ァッヵヶン", want: "ぁっゕゖん"},
		{name: "vu", in: "ヴ", want: "ゔ"},
		{name: "iteration marks", in: "ヽヾ", want: "ゝゞ"},
		{name: "long vowel mark stays", in: "スター", want: "すたー"},
		{name: "letters with no hiragana twin stay", in: "ヷ・ヺ", want: "ヷ・ヺ"},
		{name: "kanji and Latin stay", in: "光るLOVEテレビ", want: "光るLOVEてれび"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jtext.ToHiragana(tt.in); got != tt.want {
				t.Errorf("ToHiragana(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
