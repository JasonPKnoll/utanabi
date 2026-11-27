package songs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// compassSong is a song with one line, 羅針盤を見て, not analysed yet.
func compassSong() *fakeRepo {
	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Title: "歌", Lines: []Line{{Text: "羅針盤を見て"}}}
	return repo
}

func setSung(t *testing.T, repo *fakeRepo, reading string) Song {
	t.Helper()
	song, err := SetSungReading(t.Context(), repo, NewMorph(morph.New()), noMeanings(t),
		SungReadingChange{SongID: 1, Line: 0, FirstToken: 0, Reading: reading})
	if err != nil {
		t.Fatalf("SetSungReading(%q): %v", reading, err)
	}
	return song
}

func TestSungReadingShowsOverItsWord(t *testing.T) {
	t.Parallel()

	repo := compassSong()
	line := setSung(t, repo, "コンパス").Lines[0]

	if got, want := line.Ruby[0], (RubyPart{Text: "羅針盤", Reading: "コンパス"}); got != want {
		t.Errorf("furigana = %+v, want %+v", got, want)
	}
	if line.Romaji != "konpasu o mite" {
		t.Errorf("romaji = %q, want the word as it's sung", line.Romaji)
	}
	word := line.Words[0]
	if word.Sung != "コンパス" || word.SungRomaji != "konpasu" {
		t.Errorf("word sung = %q, %q, want コンパス, konpasu", word.Sung, word.SungRomaji)
	}
	if word.LemmaReading != "らしんばん" {
		t.Errorf("lemma reading = %q, want the dictionary reading kept", word.LemmaReading)
	}
	if got := line.Pieces[0].Ruby; !reflect.DeepEqual(got, []RubyPart{{Text: "羅針盤", Reading: "コンパス"}}) {
		t.Errorf("word piece = %+v, want the sung reading over it", got)
	}
}

func TestSungReadingCanBeChangedAndRemoved(t *testing.T) {
	t.Parallel()

	repo := compassSong()
	setSung(t, repo, "コンパス")
	if got := setSung(t, repo, "ｺﾝﾊﾟｽ").Lines[0].Words[0].Sung; got != "コンパス" {
		t.Errorf("half-width katakana saved as %q, want full width", got)
	}
	if got := setSung(t, repo, "ほし").Lines[0].Ruby[0]; got != (RubyPart{Text: "羅針盤", Reading: "ほし"}) {
		t.Errorf("after changing it, furigana = %+v", got)
	}
	line := setSung(t, repo, "").Lines[0]
	if line.Words[0].Sung != "" || line.Ruby[0] != (RubyPart{Text: "羅針盤", Reading: "らしんばん"}) {
		t.Errorf("after removing it, word sung %q and furigana %+v, want the dictionary reading", line.Words[0].Sung,
			line.Ruby[0])
	}
	if n := len(repo.songs[1].Lines[0].Sung); n != 0 {
		t.Errorf("%d readings stored after removing, want 0", n)
	}
}

func TestSetSungReadingRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change SungReadingChange
		field  string
	}{
		{"no song", SungReadingChange{Line: 0, Reading: "コンパス"}, "songId"},
		{"a negative line", SungReadingChange{SongID: 1, Line: -1, Reading: "コンパス"}, "line"},
		{"a negative token", SungReadingChange{SongID: 1, FirstToken: -1, Reading: "コンパス"}, "firstToken"},
		{"English", SungReadingChange{SongID: 1, Reading: "compass"}, "reading"},
		{"kanji", SungReadingChange{SongID: 1, Reading: "羅針盤"}, "reading"},
		{"only long vowel marks", SungReadingChange{SongID: 1, Reading: "ーー"}, "reading"},
		{"a line the song doesn't have", SungReadingChange{SongID: 1, Line: 5, Reading: "コンパス"}, "line"},
		{"a token that doesn't start a word", SungReadingChange{SongID: 1, FirstToken: 9, Reading: "コンパス"}, "firstToken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := SetSungReading(t.Context(), compassSong(), NewMorph(morph.New()), noMeanings(t), tt.change)
			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != tt.field {
				t.Errorf("err = %v, want invalid %s", err, tt.field)
			}
		})
	}
}

func TestSungReadingIsLeftOutWhenTheLineNoLongerSpellsTheWord(t *testing.T) {
	t.Parallel()

	line := Line{Text: "羅針盤を", Tokens: []Token{{Surface: "羅針盤", Reading: "らしんばん"}, {Surface: "を", Reading: "を"}},
		Sung: []SungReading{{FirstToken: 0, LastToken: 0, Surface: "地球", Reading: "ほし"}}}
	markSung(&line)
	if got := rubyOf(line.Tokens); got[0] != (RubyPart{Text: "羅針盤", Reading: "らしんばん"}) {
		t.Errorf("furigana = %+v, want the reading set for another word left out", got)
	}
}

func TestSungRuby(t *testing.T) {
	t.Parallel()

	tests := []struct {
		surface, reading string
		want             []RubyPart
	}{
		{"羅針盤", "コンパス", []RubyPart{{Text: "羅針盤", Reading: "コンパス"}}},
		{"想い出", "おもいで", []RubyPart{{Text: "想", Reading: "おも"}, {Text: "い"}, {Text: "出", Reading: "で"}}},
		{"ひかり", "あかり", []RubyPart{{Text: "ひかり", Reading: "あかり"}}},
	}
	for _, tt := range tests {
		if got := sungRuby(tt.surface, tt.reading); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("sungRuby(%q, %q) = %+v, want %+v", tt.surface, tt.reading, got, tt.want)
		}
	}
}
