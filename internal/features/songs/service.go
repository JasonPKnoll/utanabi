package songs

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/app"
)

// Service is the bound Wails service for songs. Every exported method here is callable by the frontend.
type Service struct {
	repo     Repository
	analyzer Analyzer
	sources  []LyricsSource
}

// NewService returns the songs service. It analyses lines with analyzer, and searches the given lyrics sources in
// order.
func NewService(repo Repository, analyzer Analyzer, sources ...LyricsSource) *Service {
	return &Service{repo: repo, analyzer: analyzer, sources: sources}
}

// List returns every song, newest first.
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	list, err := ListSongs(ctx, s.repo)
	return list, app.Fail(ctx, err)
}

// Get returns one song with its lines, their furigana, and their romaji.
func (s *Service) Get(ctx context.Context, id int64) (Song, error) {
	song, err := GetSong(ctx, s.repo, s.analyzer, id)
	return song, app.Fail(ctx, err)
}

// Add saves a song with lyrics the user typed in.
func (s *Service) Add(ctx context.Context, song NewSong) (Song, error) {
	saved, err := AddSong(ctx, s.repo, s.analyzer, song)
	return saved, app.Fail(ctx, err)
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

// AddFromSource saves a song found by Search to the library, or returns it if it's already there.
func (s *Service) AddFromSource(ctx context.Context, source, ref string) (Song, error) {
	song, err := AddFromSource(ctx, s.repo, s.analyzer, s.sources, source, ref)
	return song, app.Fail(ctx, err)
}

// Delete removes one song.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return app.Fail(ctx, DeleteSong(ctx, s.repo, id))
}
