// Package songs keeps the songs the user is learning, with their lyrics as lines.
package songs

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// SourceUser marks lyrics the user typed in themselves.
const SourceUser = "user"

const (
	maxTitleLen  = 200
	maxArtistLen = 200
	maxLyricsLen = 20000
	maxLineLen   = 500
)

// Line is one sung line. Stanza goes up by one at each verse break. StartMS is when the line starts, in milliseconds
// from the start of the song, and is set only for synced lyrics.
type Line struct {
	Position int    `json:"position"`
	Stanza   int    `json:"stanza"`
	Text     string `json:"text"`
	StartMS  *int64 `json:"startMs"`
	// Ruby is the line in pieces, with furigana over the kanji, and Romaji is the line in Latin letters. Both are
	// worked out from Tokens and never stored. A line without tokens is one piece with no reading, and no romaji.
	Ruby   []RubyPart `json:"ruby"`
	Romaji string     `json:"romaji"`
	// Tokens are the line's stored analysis. Nil means the line hasn't been analysed, or its tokens didn't fit it.
	Tokens []Token `json:"-"`
}

// RubyPart is a piece of a line, with the furigana that goes over it. Reading is empty for kana and anything else
// that needs none.
type RubyPart struct {
	Text    string `json:"text"`
	Reading string `json:"reading"`
}

// Token is one piece of a line from the analyzer: 探している is 探し, て, and いる. Surface is exactly as written, so a
// line's surfaces join back into its text. Reading is hiragana, empty for punctuation and Latin text.
type Token struct {
	Surface string
	Reading string
	POS     morph.POS
}

// Analyzer splits a line into tokens with their readings and parts of speech.
type Analyzer interface {
	Analyze(ctx context.Context, text string) ([]Token, error)
}

// Song is one song with its lines. SourceRef is the lyrics source's own ID for the song, empty for your own lyrics.
// DurationMS is 0 when the length isn't known.
type Song struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Source     string `json:"source"`
	SourceRef  string `json:"sourceRef"`
	DurationMS int64  `json:"durationMs"`
	Lines      []Line `json:"lines"`
}

// Summary is a song without its lines, for lists.
type Summary struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Source string `json:"source"`
}

// NewSong is a song the user adds with their own lyrics, pasted or typed as text: one sung line per line, and a blank
// line between verses.
type NewSong struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Lyrics string `json:"lyrics"`
}

// Repository stores songs.
type Repository interface {
	AddSong(ctx context.Context, song Song) (Song, error)
	Songs(ctx context.Context) ([]Summary, error)
	SongByID(ctx context.Context, id int64) (Song, error)
	DeleteSong(ctx context.Context, id int64) error
	// SongIDsBySource returns the library's song ID for each of refs that is already saved from source.
	SongIDsBySource(ctx context.Context, source string, refs []string) (map[string]int64, error)
	// SaveTokens stores the tokens of a saved song's lines that have none yet, and leaves lines that already have
	// tokens as they are.
	SaveTokens(ctx context.Context, songID int64, lines []Line) error
}

// AddSong validates a song the user typed in, splits its lyrics into lines, analyses them, and saves it.
func AddSong(ctx context.Context, repo Repository, analyzer Analyzer, in NewSong) (Song, error) {
	title, err := oneLine("title", in.Title, maxTitleLen)
	if err != nil {
		return Song{}, err
	}
	if title == "" {
		return Song{}, fault.Invalid("title", "is required")
	}
	artist, err := oneLine("artist", in.Artist, maxArtistLen)
	if err != nil {
		return Song{}, err
	}
	lines, err := splitLyrics(in.Lyrics)
	if err != nil {
		return Song{}, err
	}
	if err := analyzeLines(ctx, analyzer, lines); err != nil {
		return Song{}, err
	}
	song, err := repo.AddSong(ctx, Song{Title: title, Artist: artist, Source: SourceUser, Lines: lines})
	if err != nil {
		return Song{}, err
	}
	return withReadings(song), nil
}

// ListSongs returns every song, newest first.
func ListSongs(ctx context.Context, repo Repository) ([]Summary, error) {
	return repo.Songs(ctx)
}

// GetSong returns one song with its lines, their furigana, and their romaji.
func GetSong(ctx context.Context, repo Repository, analyzer Analyzer, id int64) (Song, error) {
	if id <= 0 {
		return Song{}, fault.Invalid("id", "must be positive")
	}
	return loadSong(ctx, repo, analyzer, id)
}

// DeleteSong removes one song and everything that belongs to it.
func DeleteSong(ctx context.Context, repo Repository, id int64) error {
	if id <= 0 {
		return fault.Invalid("id", "must be positive")
	}
	return repo.DeleteSong(ctx, id)
}

func oneLine(field, s string, maxLen int) (string, error) {
	s = strings.TrimSpace(jtext.Normalize(s))
	if strings.ContainsRune(s, '\n') {
		return "", fault.Invalid(field, "must be one line")
	}
	if utf8.RuneCountInString(s) > maxLen {
		return "", tooLong(field, maxLen)
	}
	return s, nil
}

// splitLyrics turns pasted lyrics into lines. Spaces at either end of a line are dropped, and any run of blank lines
// between two sung lines starts a new stanza.
func splitLyrics(lyrics string) ([]Line, error) {
	lyrics = jtext.Normalize(lyrics)
	if utf8.RuneCountInString(lyrics) > maxLyricsLen {
		return nil, tooLong("lyrics", maxLyricsLen)
	}

	var lines []Line
	stanza, gap := 0, false
	for raw := range strings.SplitSeq(lyrics, "\n") {
		text := strings.TrimSpace(raw)
		if text == "" {
			gap = true
			continue
		}
		if utf8.RuneCountInString(text) > maxLineLen {
			return nil, fault.Invalid("lyrics", "has a line over "+strconv.Itoa(maxLineLen)+" characters")
		}
		if gap && len(lines) > 0 {
			stanza++
		}
		gap = false
		lines = append(lines, Line{Position: len(lines), Stanza: stanza, Text: text})
	}
	if len(lines) == 0 {
		return nil, fault.Invalid("lyrics", "is required")
	}
	return lines, nil
}

func tooLong(field string, maxLen int) error {
	return fault.Invalid(field, "must be at most "+strconv.Itoa(maxLen)+" characters")
}
