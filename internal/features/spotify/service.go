package spotify

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// Service is the bound Wails service for Spotify. Every exported method here is callable by the frontend. With
// Spotify not set up (no client ID, or not server mode), Status says so and the rest refuse.
type Service struct {
	player  *Player
	enabled bool
}

var errNotSetUp = fault.Invalid("spotify", "isn't set up on this server")

func NewService(repo Repository, api API, enabled bool) *Service {
	return &Service{player: NewPlayer(repo, api, time.Now), enabled: enabled}
}

// Status says whether Spotify can be used here, and whether you've connected yours.
func (s *Service) Status(ctx context.Context) (Status, error) {
	if !s.enabled {
		return Status{}, nil
	}
	status, err := s.player.Status(ctx)
	return status, app.Fail(ctx, err)
}

// Disconnect forgets your Spotify tokens. Spotify's own account page can take the app's access away too.
func (s *Service) Disconnect(ctx context.Context) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.Disconnect(ctx))
}

// NowPlaying says what's playing on your Spotify, and whether it's this song.
func (s *Service) NowPlaying(ctx context.Context, songID int64) (Playback, error) {
	if !s.enabled {
		return Playback{}, app.Fail(ctx, errNotSetUp)
	}
	playback, err := s.player.NowPlaying(ctx, songID)
	return playback, app.Fail(ctx, err)
}

// Play plays the song on your Spotify from positionMs.
func (s *Service) Play(ctx context.Context, songID, positionMs int64) (PlayResult, error) {
	if !s.enabled {
		return PlayResult{}, app.Fail(ctx, errNotSetUp)
	}
	result, err := s.player.Play(ctx, songID, positionMs)
	return result, app.Fail(ctx, err)
}

// Resume carries on with what's paused on your Spotify.
func (s *Service) Resume(ctx context.Context) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.Resume(ctx))
}

// Pause pauses your Spotify.
func (s *Service) Pause(ctx context.Context) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.Pause(ctx))
}

// Seek jumps your Spotify to positionMs in what's playing.
func (s *Service) Seek(ctx context.Context, positionMs int64) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.Seek(ctx, positionMs))
}
