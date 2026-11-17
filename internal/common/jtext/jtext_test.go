package jtext_test

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text is unchanged", in: "夜の空に星が光る", want: "夜の空に星が光る"},
		{name: "half-width katakana becomes full-width", in: "ｶﾀｶﾅ", want: "カタカナ"},
		{name: "half-width voiced marks join their kana", in: "ｶﾞｷﾞﾊﾟ", want: "ガギパ"},
		{name: "half-width punctuation", in: "｢ｱｲ｣｡", want: "「アイ」。"},
		{name: "decomposed kana becomes one character", in: "が", want: "が"},
		{name: "decomposed Latin becomes one character", in: "Café", want: "Café"},
		{name: "full-width Latin is left as written", in: "ＬＯＶＥ", want: "ＬＯＶＥ"},
		{name: "circled numbers are left as written", in: "①", want: "①"},
		{name: "windows line endings", in: "一\r\n二\r三", want: "一\n二\n三"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jtext.Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsHiragana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "ひかる", want: true},
		{in: "らーめん", want: true},
		{in: "いすゞ", want: true},
		{in: "", want: false},
		{in: "ヒカル", want: false},
		{in: "光る", want: false},
		{in: "hikaru", want: false},
		{in: "ひ かる", want: false},
	}
	for _, tt := range tests {
		if got := jtext.IsHiragana(tt.in); got != tt.want {
			t.Errorf("IsHiragana(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
