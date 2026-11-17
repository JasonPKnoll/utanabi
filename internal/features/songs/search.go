package songs

import (
	"context"
	"errors"
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
type LyricsSource interface {
	Code() string
	Name() string
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
// that fails is listed in Unavailable instead of failing the search.
func SearchSongs(ctx context.Context, repo Repository, sources []LyricsSource, query string) (SearchResult, error) {
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
	ref = strings.TrimSpace(ref)
	if ref == "" || utf8.RuneCountInString(ref) > 200 {
		return Song{}, fault.Invalid("ref", "must be the source's ID for the song")
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

	title := clip(strings.TrimSpace(jtext.Normalize(fetched.Title)), maxTitleLen)
	if title == "" {
		return Song{}, fault.Invalid("title", "is missing at the source")
	}
	song, err := repo.AddSong(ctx, Song{
		Title:      title,
		Artist:     clip(strings.TrimSpace(jtext.Normalize(fetched.Artist)), maxArtistLen),
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

// clip shortens s to at most n characters, for text from a source that is longer than the library allows.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
