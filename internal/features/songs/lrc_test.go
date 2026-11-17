package songs

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func ms(n int64) *int64 { return &n }

func TestParseLRC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lrc  string
		want []Line
	}{
		{name: "lines with times",
			lrc: "[00:12.34] 夜の空に\n[00:15.00]星が光る",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "夜の空に", StartMS: ms(12340)},
				{Position: 1, Stanza: 0, Text: "星が光る", StartMS: ms(15000)},
			}},
		{name: "tenths, thousandths, and no fraction",
			lrc: "[00:01.5]一\n[00:02.250]二\n[01:03]三",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "一", StartMS: ms(1500)},
				{Position: 1, Stanza: 0, Text: "二", StartMS: ms(2250)},
				{Position: 2, Stanza: 0, Text: "三", StartMS: ms(63000)},
			}},
		{name: "a line with two times is sung twice, in time order",
			lrc: "[00:10.00][00:30.00]サビ\n[00:20.00]間",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "サビ", StartMS: ms(10000)},
				{Position: 1, Stanza: 0, Text: "間", StartMS: ms(20000)},
				{Position: 2, Stanza: 0, Text: "サビ", StartMS: ms(30000)},
			}},
		{name: "an empty timed line starts a new stanza",
			lrc: "[00:01.00]一\n[00:05.00]\n[00:09.00]二",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "一", StartMS: ms(1000)},
				{Position: 1, Stanza: 1, Text: "二", StartMS: ms(9000)},
			}},
		{name: "metadata tags and untimed lines are dropped",
			lrc: "[ar:Demo Artist]\n[ti:星の歌]\n[offset:+200]\nno time here\n[00:01.00]一",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "一", StartMS: ms(1000)},
			}},
		{name: "word timings are removed",
			lrc: "[00:01.00]<00:01.00>夜の<00:01.50>空に",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "夜の空に", StartMS: ms(1000)},
			}},
		{name: "windows line endings and half-width katakana",
			lrc: "[00:01.00]ﾗｲﾌﾞ\r\n[00:02.00]二\r\n",
			want: []Line{
				{Position: 0, Stanza: 0, Text: "ライブ", StartMS: ms(1000)},
				{Position: 1, Stanza: 0, Text: "二", StartMS: ms(2000)},
			}},
		{name: "nothing timed", lrc: "just text\n\n", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseLRC(tt.lrc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLRC() = %s, want %s", describe(got), describe(tt.want))
			}
		})
	}
}

func TestLinesFromSourceTakesVerseBreaksFromPlainLyrics(t *testing.T) {
	t.Parallel()

	got := linesFromSource("一\n\n二", "[00:01.00]一\n[00:02.00]二")
	want := []Line{
		{Position: 0, Stanza: 0, Text: "一", StartMS: ms(1000)},
		{Position: 1, Stanza: 1, Text: "二", StartMS: ms(2000)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", describe(got), describe(want))
	}
}

func TestLinesFromSourceFallsBackToPlainLyrics(t *testing.T) {
	t.Parallel()

	got := linesFromSource("一\n二", "")
	want := []Line{{Position: 0, Stanza: 0, Text: "一"}, {Position: 1, Stanza: 0, Text: "二"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", describe(got), describe(want))
	}
}

// describe prints lines with their start times, which %+v would show only as pointers.
func describe(lines []Line) string {
	var b strings.Builder
	for _, l := range lines {
		start := "none"
		if l.StartMS != nil {
			start = fmt.Sprint(*l.StartMS)
		}
		fmt.Fprintf(&b, "{%d stanza %d %q at %s}", l.Position, l.Stanza, l.Text, start)
	}
	return "[" + b.String() + "]"
}
