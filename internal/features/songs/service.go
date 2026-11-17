package songs

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/app"
)

// Service is the bound Wails service for songs. Every exported method here is callable by the frontend.
type Service struct {
	repo Repository
}

// NewService returns the songs service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns every song, newest first.
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	list, err := ListSongs(ctx, s.repo)
	return list, app.Fail(ctx, err)
}

// Get returns one song with its lyrics.
func (s *Service) Get(ctx context.Context, id int64) (Song, error) {
	song, err := GetSong(ctx, s.repo, id)
	return song, app.Fail(ctx, err)
}

// Add saves a song with lyrics the user typed in.
func (s *Service) Add(ctx context.Context, song NewSong) (Song, error) {
	saved, err := AddSong(ctx, s.repo, song)
	return saved, app.Fail(ctx, err)
}

// Delete removes one song.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return app.Fail(ctx, DeleteSong(ctx, s.repo, id))
}
