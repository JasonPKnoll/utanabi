package jtext_test

import (
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

func TestRomaji(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain syllables", in: "ひかる", want: "hikaru"},
		{name: "shi chi tsu fu ji", in: "しちつふじ", want: "shichitsufuji"},
		{name: "ぢ and づ", in: "ぢづ", want: "jizu"},
		{name: "digraph kya", in: "きゃく", want: "kyaku"},
		{name: "digraph sha", in: "しゃしん", want: "shashin"},
		{name: "digraph cha", in: "おちゃ", want: "ocha"},
		{name: "digraph ja", in: "じゃま", want: "jama"},
		{name: "digraphs with u and o", in: "りゅうきょう", want: "ryuukyou"},
		{name: "small tsu doubles the consonant", in: "きって", want: "kitte"},
		{name: "small tsu before ch is t", in: "まっち", want: "matchi"},
		{name: "small tsu before a digraph", in: "まっちゃ", want: "matcha"},
		{name: "small tsu before sh", in: "いっしょ", want: "issho"},
		{name: "small tsu at the end is dropped", in: "あっ", want: "a"},
		{name: "n before a consonant", in: "さんぽ", want: "sanpo"},
		{name: "n before a vowel", in: "げんいん", want: "gen'in"},
		{name: "n before y", in: "こんや", want: "kon'ya"},
		{name: "n at the end", in: "ほん", want: "hon"},
		{name: "long vowel mark repeats the vowel", in: "らーめん", want: "raamen"},
		{name: "long vowel mark after a digraph", in: "しょー", want: "shoo"},
		{name: "long vowel mark after n is dropped", in: "んー", want: "n"},
		{name: "katakana", in: "ラーメン", want: "raamen"},
		{name: "katakana with small tsu", in: "マッチ", want: "matchi"},
		{name: "loanword digraphs", in: "ふぁてぃゔぁしぇ", want: "fativashe"},
		{name: "vu", in: "ヴ", want: "vu"},
		{name: "を alone is o", in: "を", want: "o"},
		{name: "は inside a word is ha", in: "はな", want: "hana"},
		{name: "iteration mark", in: "いすゞ", want: "isuzu"},
		{name: "unvoiced iteration mark", in: "こゝろ", want: "kokoro"},
		{name: "small kana alone", in: "ぁ", want: "a"},
		{name: "text that isn't kana is kept", in: "光る2", want: "光ru2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jtext.Romaji(tt.in); got != tt.want {
				t.Errorf("Romaji(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRomajiLine(t *testing.T) {
	t.Parallel()

	type tok = jtext.RomajiToken
	tests := []struct {
		name   string
		tokens []tok
		want   string
	}{
		{name: "words are spaced", want: "yoru no sora", tokens: []tok{
			{Surface: "夜", Reading: "よる"}, {Surface: "の", Reading: "の", Particle: true},
			{Surface: "空", Reading: "そら"},
		}},
		{name: "particle は is wa", want: "kimi wa", tokens: []tok{
			{Surface: "君", Reading: "きみ"}, {Surface: "は", Reading: "は", Particle: true},
		}},
		{name: "particle へ is e", want: "sora e", tokens: []tok{
			{Surface: "空", Reading: "そら"}, {Surface: "へ", Reading: "へ", Particle: true},
		}},
		{name: "particle を is o", want: "koe o", tokens: []tok{
			{Surface: "声", Reading: "こえ"}, {Surface: "を", Reading: "を", Particle: true},
		}},
		{name: "は at the end of a particle is wa", want: "kimi niwa", tokens: []tok{
			{Surface: "君", Reading: "きみ"}, {Surface: "には", Reading: "には", Particle: true},
		}},
		{name: "は and へ outside a particle keep their sound", want: "hana heya", tokens: []tok{
			{Surface: "花", Reading: "はな"}, {Surface: "部屋", Reading: "へや"},
		}},
		{name: "attached tokens join", want: "tabesaserarenakatta", tokens: []tok{
			{Surface: "食べ", Reading: "たべ"}, {Surface: "させ", Reading: "させ", Attach: true},
			{Surface: "られ", Reading: "られ", Attach: true}, {Surface: "なかっ", Reading: "なかっ", Attach: true},
			{Surface: "た", Reading: "た", Attach: true},
		}},
		{name: "a token after small tsu joins it", want: "matteru", tokens: []tok{
			{Surface: "待っ", Reading: "まっ"}, {Surface: "てる", Reading: "てる"},
		}},
		{name: "small tsu doubles across attached tokens", want: "itchatta", tokens: []tok{
			{Surface: "行っ", Reading: "いっ"}, {Surface: "ちゃっ", Reading: "ちゃっ", Attach: true},
			{Surface: "た", Reading: "た", Attach: true},
		}},
		{name: "n before an attached vowel", want: "hon'ya", tokens: []tok{
			{Surface: "本", Reading: "ほん"}, {Surface: "屋", Reading: "や", Attach: true},
		}},
		{name: "katakana readings", want: "raamen", tokens: []tok{{Surface: "ラーメン", Reading: "ラーメン"}}},
		{name: "Latin text is kept and spaces between words are kept once", want: "love you kimi", tokens: []tok{
			{Surface: "love"}, {Surface: " "}, {Surface: "you"}, {Surface: " "}, {Surface: "君", Reading: "きみ"},
		}},
		{name: "Latin pieces written together stay together", want: "I'm here", tokens: []tok{
			{Surface: "I"}, {Surface: "'"}, {Surface: "m"}, {Surface: " "}, {Surface: "here"},
		}},
		{name: "Japanese punctuation", want: `"sayonara", mata ashita!`, tokens: []tok{
			{Surface: "「"}, {Surface: "さよなら", Reading: "さよなら"}, {Surface: "」"}, {Surface: "、"},
			{Surface: "また", Reading: "また"}, {Surface: "明日", Reading: "あした"}, {Surface: "！"},
		}},
		{name: "a full-width space and a middle dot separate words", want: "sora umi", tokens: []tok{
			{Surface: "　"}, {Surface: "空", Reading: "そら"}, {Surface: "・"}, {Surface: "海", Reading: "うみ"},
		}},
		{name: "a number before a counter", want: "2 nin", tokens: []tok{
			{Surface: "2"}, {Surface: "人", Reading: "にん"},
		}},
		{name: "a symbol it doesn't know is kept", want: "arigatou ♪", tokens: []tok{
			{Surface: "ありがとう", Reading: "ありがとう"}, {Surface: "♪"},
		}},
		{name: "no tokens", want: "", tokens: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jtext.RomajiLine(tt.tokens); got != tt.want {
				t.Errorf("RomajiLine = %q, want %q", got, tt.want)
			}
		})
	}
}
