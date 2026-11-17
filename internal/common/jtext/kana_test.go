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
		{name: "katakana becomes hiragana", in: "カタカナ", want: "かたかな"},
		{name: "short katakana word", in: "カラ", want: "から"},
		{name: "long vowel mark stays", in: "ラーメン", want: "らーめん"},
		{name: "long vowel mark at the end", in: "スター", want: "すたー"},
		{name: "vu", in: "ヴ", want: "ゔ"},
		{name: "vu with a small vowel", in: "ヴァイオリン", want: "ゔぁいおりん"},
		{name: "small tsu and ya", in: "マッチャ", want: "まっちゃ"},
		{name: "small ka and ke", in: "ヵヶ", want: "ゕゖ"},
		{name: "small kana and the first and last letters", in: "ァッヵヶン", want: "ぁっゕゖん"},
		{name: "iteration marks", in: "ヽヾ", want: "ゝゞ"},
		{name: "letters with no hiragana twin stay", in: "ヷ・ヺ", want: "ヷ・ヺ"},
		{name: "single letter with no hiragana twin", in: "ヷ", want: "ヷ"},
		{name: "kanji, Latin, and punctuation stay", in: "光るLOVE、", want: "光るLOVE、"},
		{name: "kanji and Latin stay around katakana", in: "光るLOVEテレビ", want: "光るLOVEてれび"},
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
