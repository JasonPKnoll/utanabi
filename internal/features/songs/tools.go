package songs

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

// The admins' song tools, for testing: finding songs the timing databases have, adding one with its word times, and
// adding one of yours again fresh. Each is for an admin only.

// TimedSong is a song a timing database has word times for. Sheet is its ID there.
type TimedSong struct {
	Sheet  string `json:"sheet"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// ToolResult is the song a tool added: its ID, title, how many lines it has, and how many have word times, and from
// where.
type ToolResult struct {
	SongID int64  `json:"songId"`
	Title  string `json:"title"`
	Lines  int    `json:"lines"`
	Timed  int    `json:"timed"`
	Source string `json:"source"`
}

// The most songs a search of the timing databases returns, and how many LRCLIB versions of one are read to find the
// one its word times fit.
const (
	maxTimedSongs    = 50
	maxVersionsTried = 4
)

// backingTrack is a title for a version without the singing, which LRCLIB has synced lyrics for as karaoke: adding it
// would match the backing track on Spotify too.
var backingTrack = regexp.MustCompile(`(?i)\b(inst|instrumental|karaoke|off ?vocal)\b|カラオケ|オフボーカル|インスト`)

// Toolkit is what the tools work with: the song library and its analysis, the lyrics sources, and the timing
// databases.
type Toolkit struct {
	Repo     Repository
	Analyzer Analyzer
	Dict     Dictionary
	Sources  []LyricsSource
	Timings  []TimingSource
}

// TimedSongs returns the songs the timing databases have whose title or artist has query in it, compared as letters
// and digits; with no query, the ones added last. At most 50.
func (k Toolkit) TimedSongs(ctx context.Context, query string) ([]TimedSong, error) {
	if _, err := identity.Admin(ctx); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(query) > maxQueryLen {
		return nil, tooLong("query", maxQueryLen)
	}
	want := jtext.Plain(query)
	out := []TimedSong{}
	for _, src := range k.Timings {
		refs, err := src.Sheets(ctx)
		if err != nil {
			return nil, err
		}
		// The index lists sheets oldest first, so the ones added last are at its end.
		for _, ref := range slices.Backward(refs) {
			if want != "" && !strings.Contains(jtext.Plain(ref.Title+" "+ref.Artist), want) {
				continue
			}
			out = append(out, TimedSong{Sheet: ref.ID, Title: ref.Title, Artist: ref.Artist})
			if len(out) == maxTimedSongs {
				return out, nil
			}
		}
	}
	return out, nil
}

// AddTimedSong adds the song a timing database has as sheet, with LRCLIB's lyrics: of the synced versions LRCLIB
// finds for its title and artist, the one the sheet spells out most of. Its word times come from the sheet.
func (k Toolkit) AddTimedSong(ctx context.Context, sheetID string) (ToolResult, error) {
	if _, err := identity.Admin(ctx); err != nil {
		return ToolResult{}, err
	}
	src, ref, err := k.sheetRef(ctx, sheetID)
	if err != nil {
		return ToolResult{}, err
	}
	sheet, err := src.Sheet(ctx, ref)
	if err != nil {
		return ToolResult{}, err
	}

	var best Match
	bestTimed, bestScore := 0, 0
	for _, lyrics := range k.Sources {
		matches, err := lyrics.Search(ctx, strings.TrimSpace(ref.Title+" "+ref.Artist))
		if err != nil {
			return ToolResult{}, err
		}
		tried := 0
		for _, m := range matches {
			if !m.Synced || tried == maxVersionsTried || backingTrack.MatchString(m.Title) {
				continue
			}
			tried++
			fetched, err := lyrics.Lyrics(ctx, m.SourceRef)
			if err != nil {
				return ToolResult{}, err
			}
			// A version titled as the song wins a tie, over one with more in its title.
			n := len(lineUpSheet(linesFromSource(fetched.Plain, fetched.Synced), sheet))
			score := 2 * n
			if jtext.Plain(m.Title) == jtext.Plain(ref.Title) {
				score++
			}
			if score > bestScore {
				best, bestTimed, bestScore = m, n, score
			}
		}
	}
	if bestTimed < minLinesTimed {
		return ToolResult{}, fault.Invalid("song", "has no synced lyrics at LRCLIB that its word times fit")
	}
	song, err := AddFromSource(ctx, k.Repo, k.Analyzer, k.Dict, k.Sources, best.Source, best.SourceRef)
	if err != nil {
		return ToolResult{}, err
	}
	saved, err := k.Repo.SongByID(ctx, song.ID)
	if err != nil {
		return ToolResult{}, err
	}
	lines := lineUpSheet(saved.Lines, sheet)
	if err := k.Repo.SetWordTimes(ctx, song.ID, src.Code(), lines); err != nil {
		return ToolResult{}, err
	}
	return k.result(ctx, song.ID, src.Name())
}

// ReaddSong removes one of your songs from every library on the server, and everything that belongs to it, then adds
// it again from its lyrics source, analysed fresh, and looks its word times up again. Your own lyrics can't be
// fetched again, so a song with them is refused.
func (k Toolkit) ReaddSong(ctx context.Context, songID int64) (ToolResult, error) {
	if _, err := identity.Admin(ctx); err != nil {
		return ToolResult{}, err
	}
	if songID <= 0 {
		return ToolResult{}, fault.Invalid("songId", "must be positive")
	}
	song, err := k.Repo.SongByID(ctx, songID)
	if err != nil {
		return ToolResult{}, err
	}
	if song.Source == SourceUser || findSource(k.Sources, song.Source) == nil {
		return ToolResult{}, fault.Invalid("song", "has lyrics that can't be fetched again")
	}
	if err := k.Repo.PurgeSong(ctx, songID); err != nil {
		return ToolResult{}, err
	}
	added, err := AddFromSource(ctx, k.Repo, k.Analyzer, k.Dict, k.Sources, song.Source, song.SourceRef)
	if err != nil {
		return ToolResult{}, err
	}
	source := ""
	if len(k.Timings) > 0 && slices.ContainsFunc(added.Lines, func(l Line) bool { return l.StartMS != nil }) {
		found, err := FindWordTimes(ctx, k.Repo, k.Analyzer, k.Timings, added.ID)
		if err != nil {
			return ToolResult{}, err
		}
		source = found.Source
	}
	return k.result(ctx, added.ID, source)
}

func (k Toolkit) sheetRef(ctx context.Context, sheetID string) (TimingSource, SheetRef, error) {
	for _, src := range k.Timings {
		refs, err := src.Sheets(ctx)
		if err != nil {
			return nil, SheetRef{}, err
		}
		if i := slices.IndexFunc(refs, func(r SheetRef) bool { return r.ID == sheetID }); i >= 0 {
			return src, refs[i], nil
		}
	}
	return nil, SheetRef{}, fault.ErrNotFound
}

func (k Toolkit) result(ctx context.Context, songID int64, source string) (ToolResult, error) {
	song, err := k.Repo.SongByID(ctx, songID)
	if err != nil {
		return ToolResult{}, err
	}
	out := ToolResult{SongID: song.ID, Title: song.Title, Lines: len(song.Lines), Source: source}
	for _, l := range song.Lines {
		if len(l.Times) > 0 {
			out.Timed++
		}
	}
	return out, nil
}
