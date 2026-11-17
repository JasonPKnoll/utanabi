// Package songs keeps the songs the user is learning, with their lyrics as lines.
package songs

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

// SourceUser marks lyrics the user typed in themselves.
const SourceUser = "user"

const (
	maxTitleLen  = 200
	maxArtistLen = 200
	maxLyricsLen = 20000
	maxLineLen   = 500
)

// Line is one sung line. Stanza goes up by one at each verse break.
type Line struct {
	Position int    `json:"position"`
	Stanza   int    `json:"stanza"`
	Text     string `json:"text"`
}

// Song is one song with its lines.
type Song struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Source string `json:"source"`
	Lines  []Line `json:"lines"`
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
}

// AddSong validates a song the user typed in, splits its lyrics into lines, and saves it.
func AddSong(ctx context.Context, repo Repository, in NewSong) (Song, error) {
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
	return repo.AddSong(ctx, Song{Title: title, Artist: artist, Source: SourceUser, Lines: lines})
}

// ListSongs returns every song, newest first.
func ListSongs(ctx context.Context, repo Repository) ([]Summary, error) {
	return repo.Songs(ctx)
}

// GetSong returns one song with its lines.
func GetSong(ctx context.Context, repo Repository, id int64) (Song, error) {
	if id <= 0 {
		return Song{}, fault.Invalid("id", "must be positive")
	}
	return repo.SongByID(ctx, id)
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
