// Package songs keeps the songs the user is learning, with their lyrics.
package songs

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// SourceUser marks lyrics the user typed in themselves.
const SourceUser = "user"

const (
	maxTitleLen  = 200
	maxArtistLen = 200
	maxLyricsLen = 20000
)

// Song is one song with its full lyrics.
type Song struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Lyrics string `json:"lyrics"`
	Source string `json:"source"`
}

// Summary is a song without its lyrics, for lists.
type Summary struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Source string `json:"source"`
}

// NewSong is a song the user adds with their own lyrics.
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

// AddSong validates a song the user typed in and saves it.
func AddSong(ctx context.Context, repo Repository, in NewSong) (Song, error) {
	song := Song{
		Title:  strings.TrimSpace(in.Title),
		Artist: strings.TrimSpace(in.Artist),
		Lyrics: normalizeLyrics(in.Lyrics),
		Source: SourceUser,
	}
	if err := required("title", song.Title, maxTitleLen); err != nil {
		return Song{}, err
	}
	if utf8.RuneCountInString(song.Artist) > maxArtistLen {
		return Song{}, tooLong("artist", maxArtistLen)
	}
	if err := required("lyrics", song.Lyrics, maxLyricsLen); err != nil {
		return Song{}, err
	}
	return repo.AddSong(ctx, song)
}

// ListSongs returns every song, newest first.
func ListSongs(ctx context.Context, repo Repository) ([]Summary, error) {
	return repo.Songs(ctx)
}

// GetSong returns one song with its lyrics.
func GetSong(ctx context.Context, repo Repository, id int64) (Song, error) {
	if id <= 0 {
		return Song{}, fault.Invalid("id", "must be positive")
	}
	return repo.SongByID(ctx, id)
}

// DeleteSong removes one song.
func DeleteSong(ctx context.Context, repo Repository, id int64) error {
	if id <= 0 {
		return fault.Invalid("id", "must be positive")
	}
	return repo.DeleteSong(ctx, id)
}

// normalizeLyrics keeps the user's line breaks but drops Windows line endings, trailing spaces on each line, and
// blank lines at the start and end.
func normalizeLyrics(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t　")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func required(field, value string, maxLen int) error {
	if value == "" {
		return fault.Invalid(field, "is required")
	}
	if utf8.RuneCountInString(value) > maxLen {
		return tooLong(field, maxLen)
	}
	return nil
}

func tooLong(field string, maxLen int) error {
	return fault.Invalid(field, "must be at most "+strconv.Itoa(maxLen)+" characters")
}
