package songs

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

const maxQueryLen = 200

// Match is one song a lyrics source found. SongID is set when the song is already in the library.
type Match struct {
	Source     string `json:"source"`
	SourceName string `json:"sourceName"`
	SourceRef  string `json:"sourceRef"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"durationMs"`
	Synced     bool   `json:"synced"`
	SongID     int64  `json:"songId"`
}

// Fetched is a match with its lyrics: plain text, and LRC text when the source has synced lyrics.
type Fetched struct {
	Match
	Plain  string
	Synced string
}

// LyricsSource is an online place to find song lyrics, such as LRCLIB. Search returns only songs that have lyrics.
// CanonicalRef turns a song ID into the one form the source uses (LRCLIB's "011" is "11"), or fails for an ID the
// source can't have, so the library never saves the same song twice under different spellings.
type LyricsSource interface {
	Code() string
	Name() string
	CanonicalRef(ref string) (string, error)
	Search(ctx context.Context, query string) ([]Match, error)
	Lyrics(ctx context.Context, ref string) (Fetched, error)
}

// SearchResult is what a search found. Unavailable names the sources that didn't answer; the others' matches are
// still there.
type SearchResult struct {
	Matches     []Match  `json:"matches"`
	Unavailable []string `json:"unavailable"`

	// failures are the errors behind Unavailable, for the service to log. They never reach the frontend.
	failures []error
}

// SearchSongs asks each source in turn for songs matching query, and marks the ones already in the library. A source
// that fails is listed in Unavailable instead of failing the search. With no sources on, search is turned off.
func SearchSongs(ctx context.Context, repo Repository, sources []LyricsSource, query string) (SearchResult, error) {
	if len(sources) == 0 {
		return SearchResult{}, fault.Invalid("search", "is turned off")
	}
	query = strings.TrimSpace(jtext.Normalize(query))
	if query == "" {
		return SearchResult{}, fault.Invalid("query", "is required")
	}
	if utf8.RuneCountInString(query) > maxQueryLen {
		return SearchResult{}, tooLong("query", maxQueryLen)
	}

	result := SearchResult{Matches: []Match{}, Unavailable: []string{}}
	for _, src := range sources {
		matches, err := src.Search(ctx, query)
		if err != nil {
			if ctx.Err() != nil {
				return SearchResult{}, err
			}
			result.Unavailable = append(result.Unavailable, src.Name())
			result.failures = append(result.failures, err)
			continue
		}
		if err := markSaved(ctx, repo, src.Code(), matches); err != nil {
			return SearchResult{}, err
		}
		result.Matches = append(result.Matches, matches...)
	}
	return result, nil
}

// AddFromSource saves a song from a lyrics source to the library, with synced lyrics when the source has them. A song
// already in the library is returned as it is, so adding twice is safe.
func AddFromSource(ctx context.Context, repo Repository, sources []LyricsSource, source, ref string) (Song, error) {
	src := findSource(sources, source)
	if src == nil {
		return Song{}, fault.Invalid("source", "is not available")
	}
	ref, err := src.CanonicalRef(strings.TrimSpace(ref))
	if err != nil {
		return Song{}, err
	}

	if song, ok, err := savedSong(ctx, repo, source, ref); err != nil || ok {
		return song, err
	}

	fetched, err := src.Lyrics(ctx, ref)
	if err != nil {
		return Song{}, err
	}
	lines := linesFromSource(fetched.Plain, fetched.Synced)
	if len(lines) == 0 {
		return Song{}, fault.Invalid("lyrics", "aren't available for this song")
	}
	if lyricsLen(lines) > maxLyricsLen {
		return Song{}, fault.Invalid("lyrics", "are too long to save (over "+strconv.Itoa(maxLyricsLen)+" characters)")
	}

	title := sourceText(fetched.Title, maxTitleLen)
	if title == "" {
		return Song{}, fault.Invalid("title", "is missing at the source")
	}
	song, err := repo.AddSong(ctx, Song{
		Title:      title,
		Artist:     sourceText(fetched.Artist, maxArtistLen),
		Source:     source,
		SourceRef:  ref,
		DurationMS: fetched.DurationMS,
		Lines:      lines,
	})
	if errors.Is(err, fault.ErrConflict) {
		// Saved by another call between the check above and now.
		if saved, ok, lookupErr := savedSong(ctx, repo, source, ref); lookupErr == nil && ok {
			return saved, nil
		}
	}
	return song, err
}

// linesFromSource prefers synced lyrics, for the timings, and takes the verse breaks from the plain lyrics when both
// have the same lines.
func linesFromSource(plain, synced string) []Line {
	plainLines, err := splitLyrics(plain)
	if err != nil {
		plainLines = nil
	}
	syncedLines := parseLRC(synced)
	if len(syncedLines) == 0 {
		return plainLines
	}
	if len(plainLines) == len(syncedLines) {
		same := true
		for i := range plainLines {
			if plainLines[i].Text != syncedLines[i].Text {
				same = false
				break
			}
		}
		if same {
			for i := range syncedLines {
				syncedLines[i].Stanza = plainLines[i].Stanza
			}
		}
	}
	return syncedLines
}

func markSaved(ctx context.Context, repo Repository, source string, matches []Match) error {
	if len(matches) == 0 {
		return nil
	}
	refs := make([]string, len(matches))
	for i, m := range matches {
		refs[i] = m.SourceRef
	}
	saved, err := repo.SongIDsBySource(ctx, source, refs)
	if err != nil {
		return err
	}
	for i := range matches {
		matches[i].SongID = saved[matches[i].SourceRef]
	}
	return nil
}

func savedSong(ctx context.Context, repo Repository, source, ref string) (Song, bool, error) {
	saved, err := repo.SongIDsBySource(ctx, source, []string{ref})
	if err != nil {
		return Song{}, false, err
	}
	id, ok := saved[ref]
	if !ok {
		return Song{}, false, nil
	}
	song, err := repo.SongByID(ctx, id)
	return song, err == nil, err
}

func findSource(sources []LyricsSource, code string) LyricsSource {
	for _, src := range sources {
		if src.Code() == code {
			return src
		}
	}
	return nil
}

// sourceText cleans a title or artist from a source the way a typed one is cleaned, but fixes what a person would be
// asked to fix: a line break becomes a space, and text over the limit is shortened.
func sourceText(s string, maxLen int) string {
	s = strings.Join(strings.Split(jtext.Normalize(s), "\n"), " ")
	return clip(strings.TrimSpace(s), maxLen)
}

// lyricsLen counts the characters in a song's lines, the same measure as the limit on lyrics a person types.
func lyricsLen(lines []Line) int {
	n := 0
	for _, l := range lines {
		n += utf8.RuneCountInString(l.Text)
	}
	return n
}

// clip shortens s to at most n characters, for text from a source that is longer than the library allows.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
