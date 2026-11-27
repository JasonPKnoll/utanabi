package songs

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

const maxSungLen = 100

// SungReading is how a word of a line is sung when that isn't how it's written: gikun, where a singer writes 羅針盤
// ("compass") and sings コンパス, or writes 地球 and sings ほし. No analyzer can hear that, so you set it. Line is the
// line's position, FirstToken to LastToken the word's tokens, and Surface its text as written, so a reading set
// before the line was analysed again is only used while the tokens still spell the word. Reading is kana, hiragana or
// katakana, as it should show over the word.
type SungReading struct {
	Line       int
	FirstToken int
	LastToken  int
	Surface    string
	Reading    string
}

// SungReadingChange sets how the word starting at FirstToken in line Line of a song is sung. An empty Reading
// removes it, so the word shows its dictionary reading again.
type SungReadingChange struct {
	SongID     int64  `json:"songId"`
	Line       int    `json:"line"`
	FirstToken int    `json:"firstToken"`
	Reading    string `json:"reading"`
}

// SetSungReading saves how a word is sung and returns the song as it shows now.
func SetSungReading(ctx context.Context, repo Repository, analyzer Analyzer, dict Dictionary, change SungReadingChange) (
	Song, error,
) {
	if change.SongID <= 0 {
		return Song{}, fault.Invalid("songId", "must be positive")
	}
	if change.Line < 0 {
		return Song{}, fault.Invalid("line", "must not be negative")
	}
	if change.FirstToken < 0 {
		return Song{}, fault.Invalid("firstToken", "must not be negative")
	}
	reading := jtext.Normalize(strings.TrimSpace(change.Reading))
	if reading != "" && (!jtext.IsKana(reading) || strings.Trim(reading, "ー") == "") {
		return Song{}, fault.Invalid("reading", "must be written in hiragana or katakana")
	}
	if utf8.RuneCountInString(reading) > maxSungLen {
		return Song{}, fault.Invalid("reading", "must be at most 100 characters")
	}

	song, err := loadSong(ctx, repo, analyzer, dict, change.SongID)
	if err != nil {
		return Song{}, err
	}
	var line *Line
	for i := range song.Lines {
		if song.Lines[i].Position == change.Line {
			line = &song.Lines[i]
		}
	}
	if line == nil {
		return Song{}, fault.Invalid("line", "is not a line of this song")
	}
	word := -1
	for i, a := range line.Analysis {
		if a.FirstToken == change.FirstToken && a.LastToken < len(line.Tokens) {
			word = i
		}
	}
	if word < 0 {
		return Song{}, fault.Invalid("firstToken", "is not the start of a word in this line")
	}
	a := line.Analysis[word]
	sung := SungReading{Line: change.Line, FirstToken: a.FirstToken, LastToken: a.LastToken,
		Surface: surfaceOf(line.Tokens[a.FirstToken : a.LastToken+1]), Reading: reading}
	if err := repo.SetSungReading(ctx, change.SongID, sung); err != nil {
		return Song{}, err
	}
	return loadSong(ctx, repo, analyzer, dict, change.SongID)
}

func surfaceOf(tokens []Token) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.Surface)
	}
	return b.String()
}

// markSung puts each of a line's sung readings on the first token of its word, for the furigana and romaji. A
// reading whose tokens no longer spell its word, or that overlaps one already placed, is left out.
func markSung(line *Line) {
	for _, s := range line.Sung {
		if s.FirstToken < 0 || s.LastToken < s.FirstToken || s.LastToken >= len(line.Tokens) {
			continue
		}
		span := line.Tokens[s.FirstToken : s.LastToken+1]
		if surfaceOf(span) != s.Surface || overlapsSung(line.Tokens, s.FirstToken, s.LastToken) {
			continue
		}
		line.Tokens[s.FirstToken].sung, line.Tokens[s.FirstToken].sungEnd = s.Reading, s.LastToken
	}
}

func overlapsSung(tokens []Token, first, last int) bool {
	for i, t := range tokens {
		if t.sung != "" && i <= last && t.sungEnd >= first {
			return true
		}
	}
	return false
}

// sungRuby is the furigana for a word with a sung reading: a hiragana reading is lined up with the word's kana
// (想い出 sung おもいで keeps い and で bare), and a katakana one, or one over a word written in kana, goes over the
// whole word as written.
func sungRuby(surface, reading string) []RubyPart {
	if jtext.IsHiragana(reading) && !jtext.IsKana(surface) {
		var parts []RubyPart
		for _, p := range jtext.Ruby(surface, reading) {
			parts = append(parts, RubyPart{Text: p.Text, Reading: p.Reading})
		}
		return parts
	}
	return []RubyPart{{Text: surface, Reading: reading}}
}
