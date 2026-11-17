package jtext_test

import (
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

func TestRuby(t *testing.T) {
	t.Parallel()

	type part = jtext.RubyPart
	tests := []struct {
		name    string
		surface string
		reading string
		want    []part
	}{
		{name: "kana at the end is left bare", surface: "光る", reading: "ひかる",
			want: []part{{Text: "光", Reading: "ひか"}, {Text: "る"}}},
		{name: "a verb stem", surface: "探し", reading: "さがし",
			want: []part{{Text: "探", Reading: "さが"}, {Text: "し"}}},
		{name: "one kanji", surface: "夜", reading: "よる", want: []part{{Text: "夜", Reading: "よる"}}},
		{name: "kana on both sides", surface: "お母さん", reading: "おかあさん",
			want: []part{{Text: "お"}, {Text: "母", Reading: "かあ"}, {Text: "さん"}}},
		{name: "a short stem", surface: "食べ", reading: "たべ",
			want: []part{{Text: "食", Reading: "た"}, {Text: "べ"}}},
		{name: "a kanji word read as a whole", surface: "一人", reading: "ひとり",
			want: []part{{Text: "一人", Reading: "ひとり"}}},
		{name: "katakana needs no furigana", surface: "ラーメン", reading: "らーめん",
			want: []part{{Text: "ラーメン"}}},
		{name: "a digit and a kanji share one reading", surface: "2人", reading: "ふたり",
			want: []part{{Text: "2人", Reading: "ふたり"}}},
		{name: "the iteration mark stays with its kanji", surface: "時々", reading: "ときどき",
			want: []part{{Text: "時々", Reading: "ときどき"}}},
		{name: "two kanji runs", surface: "取り扱い", reading: "とりあつかい",
			want: []part{{Text: "取", Reading: "と"}, {Text: "り"}, {Text: "扱", Reading: "あつか"}, {Text: "い"}}},
		{name: "a kanji whose reading starts like the kana after it", surface: "下さい", reading: "ください",
			want: []part{{Text: "下", Reading: "くだ"}, {Text: "さい"}}},
		{name: "katakana in the word matches hiragana in the reading", surface: "ドキ胸", reading: "どきむね",
			want: []part{{Text: "ドキ"}, {Text: "胸", Reading: "むね"}}},
		{name: "a katakana reading becomes hiragana", surface: "空", reading: "ソラ",
			want: []part{{Text: "空", Reading: "そら"}}},
		{name: "kana that isn't in the reading puts the reading over the whole word", surface: "行く", reading: "いった",
			want: []part{{Text: "行く", Reading: "いった"}}},
		{name: "a reading too short for the kanji", surface: "お母さん", reading: "おさん",
			want: []part{{Text: "お母さん", Reading: "おさん"}}},
		{name: "no reading", surface: "love", reading: "", want: []part{{Text: "love"}}},
		{name: "kanji with no reading", surface: "空", reading: "", want: []part{{Text: "空"}}},
		{name: "empty", surface: "", reading: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := jtext.Ruby(tt.surface, tt.reading); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Ruby(%q, %q) = %+v, want %+v", tt.surface, tt.reading, got, tt.want)
			}
		})
	}
}
