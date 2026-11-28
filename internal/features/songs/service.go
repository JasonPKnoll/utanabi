package songs

import (
	"context"
	"slices"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

// Service is the bound Wails service for songs. Every exported method here is callable by the frontend.
type Service struct {
	repo     Repository
	analyzer Analyzer
	dict     Dictionary
	timings  []TimingSource
	sources  []LyricsSource
}

// NewService returns the songs service. It analyses lines with analyzer, explains their words with dict, looks word
// times up in timings, and searches the given lyrics sources in order.
func NewService(
	repo Repository, analyzer Analyzer, dict Dictionary, timings []TimingSource, sources ...LyricsSource,
) *Service {
	return &Service{repo: repo, analyzer: analyzer, dict: dict, timings: timings, sources: sources}
}

// List returns every song, newest first.
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	list, err := ListSongs(ctx, s.repo)
	return list, app.Fail(ctx, err)
}

// Get returns one song with its lines: their furigana, romaji, words, and word-by-word gloss.
func (s *Service) Get(ctx context.Context, id int64) (Song, error) {
	song, err := GetSong(ctx, s.repo, s.analyzer, s.dict, id)
	return song, app.Fail(ctx, err)
}

// Add saves a song with lyrics the user typed in.
func (s *Service) Add(ctx context.Context, song NewSong) (Song, error) {
	saved, err := AddSong(ctx, s.repo, s.analyzer, s.dict, song)
	return saved, app.Fail(ctx, err)
}

// LookupWord returns every meaning of a word, for the word sheet. entryID is the entry the word in the song points
// at, or 0.
func (s *Service) LookupWord(ctx context.Context, lemma, reading string, entryID int64) (Meanings, error) {
	meanings, err := LookupWord(ctx, s.dict, lemma, reading, entryID)
	return meanings, app.Fail(ctx, err)
}

// LookupEntries returns the meanings of the dictionary entries with the given IDs in one call, so a song's words can
// open their sheets without asking again.
func (s *Service) LookupEntries(ctx context.Context, ids []int64) ([]Meanings, error) {
	meanings, err := LookupEntries(ctx, s.dict, ids)
	return meanings, app.Fail(ctx, err)
}

// SearchSources returns the names of the online lyrics sources that are on, in the order search asks them. Empty
// means online search is turned off.
func (s *Service) SearchSources() []string {
	names := make([]string, len(s.sources))
	for i, src := range s.sources {
		names[i] = src.Name()
	}
	return names
}

// Search finds songs with lyrics at the online lyrics sources.
func (s *Service) Search(ctx context.Context, query string) (SearchResult, error) {
	result, err := SearchSongs(ctx, s.repo, s.sources, query)
	for _, failure := range result.failures {
		app.Note(ctx, failure)
	}
	return result, app.Fail(ctx, err)
}

// AddFromSource saves a song found by Search to the library, or returns it if it's already there. When its lyrics
// are synced, it looks its word times up too; a song the timing sources don't have, or can't be reached for, is
// saved all the same.
func (s *Service) AddFromSource(ctx context.Context, source, ref string) (Song, error) {
	song, err := AddFromSource(ctx, s.repo, s.analyzer, s.dict, s.sources, source, ref)
	if err != nil {
		return song, app.Fail(ctx, err)
	}
	if len(s.timings) > 0 && slices.ContainsFunc(song.Lines, func(l Line) bool { return l.StartMS != nil }) &&
		!slices.ContainsFunc(song.Lines, func(l Line) bool { return len(l.Times) > 0 }) {
		if _, err := FindWordTimes(ctx, s.repo, s.analyzer, s.timings, song.ID); err != nil {
			app.Note(ctx, err)
		}
	}
	return song, nil
}

// TimingSources returns the names of the word timing databases that are on, in the order they're asked. Empty means
// word times are only ever yours.
func (s *Service) TimingSources() []string {
	names := make([]string, len(s.timings))
	for i, src := range s.timings {
		names[i] = src.Name()
	}
	return names
}

// FindWordTimes looks the song's word times up in the timing databases and saves them, for the lines you haven't
// timed yourself. It says how many lines got times, and from where; none when no database has the song.
func (s *Service) FindWordTimes(ctx context.Context, songID int64) (FoundTimes, error) {
	found, err := FindWordTimes(ctx, s.repo, s.analyzer, s.timings, songID)
	return found, app.Fail(ctx, err)
}

// SetLineTimes saves when the words of one line start, as you tapped them along with the song, or removes the
// line's word times when there are none. It returns the times as saved.
func (s *Service) SetLineTimes(ctx context.Context, change LineTimesChange) ([]WordTime, error) {
	times, err := SetLineTimes(ctx, s.repo, change)
	return times, app.Fail(ctx, err)
}

// ClearWordTimes removes every word time of a song, yours and found ones.
func (s *Service) ClearWordTimes(ctx context.Context, songID int64) error {
	return app.Fail(ctx, ClearWordTimes(ctx, s.repo, songID))
}

// SetSungReading sets how a word of a song is sung when that isn't how it's written, such as 羅針盤 sung コンパス, or
// removes it when the reading is empty. It returns the song as it shows now.
func (s *Service) SetSungReading(ctx context.Context, change SungReadingChange) (Song, error) {
	song, err := SetSungReading(ctx, s.repo, s.analyzer, s.dict, change)
	return song, app.Fail(ctx, err)
}

// Delete removes one song.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return app.Fail(ctx, DeleteSong(ctx, s.repo, id))
}

// Tools is the bound Wails service for the admins' song tools, for testing, on only with UTANABI_SONG_TOOLS on a
// server. Every method checks the signed-in user is an admin.
type Tools struct {
	kit Toolkit
	on  bool
}

// NewTools returns the song tools, working with kit, and on only when on is true.
func NewTools(kit Toolkit, on bool) *Tools {
	return &Tools{kit: kit, on: on}
}

// Available says whether the song tools are on and the signed-in user is an admin, so the app can offer them.
func (t *Tools) Available(ctx context.Context) bool {
	_, err := identity.Admin(ctx)
	return t.on && err == nil
}

// TimedSongs lists the songs the word timing databases have whose title or artist has query in it.
func (t *Tools) TimedSongs(ctx context.Context, query string) ([]TimedSong, error) {
	if !t.on {
		return nil, app.Fail(ctx, fault.ErrNotFound)
	}
	songs, err := t.kit.TimedSongs(ctx, query)
	return songs, app.Fail(ctx, err)
}

// AddTimedSong adds a song a word timing database has, with LRCLIB's lyrics that its word times fit, and those times.
func (t *Tools) AddTimedSong(ctx context.Context, sheet string) (ToolResult, error) {
	if !t.on {
		return ToolResult{}, app.Fail(ctx, fault.ErrNotFound)
	}
	result, err := t.kit.AddTimedSong(ctx, sheet)
	return result, app.Fail(ctx, err)
}

// ReaddSong builds one of your songs again as if just added: lyrics fetched again, analysed fresh, and word times
// looked up again. It keeps the song's offsets and matched track and video.
func (t *Tools) ReaddSong(ctx context.Context, songID int64) (ToolResult, error) {
	if !t.on {
		return ToolResult{}, app.Fail(ctx, fault.ErrNotFound)
	}
	result, err := t.kit.ReaddSong(ctx, songID)
	return result, app.Fail(ctx, err)
}
