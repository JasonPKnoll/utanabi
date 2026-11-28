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

func NewService(repo Repository, api API, romaji Romanizer, enabled bool) *Service {
	return &Service{player: NewPlayer(repo, api, romaji, time.Now), enabled: enabled}
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

// Play plays the song on your Spotify from positionMs: on deviceId, such as this page's own player, or with none
// given, on the device you're most likely listening on.
func (s *Service) Play(ctx context.Context, songID, positionMs int64, deviceID string) (PlayResult, error) {
	if !s.enabled {
		return PlayResult{}, app.Fail(ctx, errNotSetUp)
	}
	result, err := s.player.Play(ctx, songID, positionMs, deviceID)
	return result, app.Fail(ctx, err)
}

// Devices lists your Spotify devices that are open now, such as your phone and your computer, to choose where to play.
func (s *Service) Devices(ctx context.Context) ([]Device, error) {
	if !s.enabled {
		return nil, app.Fail(ctx, errNotSetUp)
	}
	devices, err := s.player.Devices(ctx)
	return devices, app.Fail(ctx, err)
}

// BrowserToken is an access token for this page's own Spotify player, which plays in the browser.
func (s *Service) BrowserToken(ctx context.Context) (BrowserToken, error) {
	if !s.enabled {
		return BrowserToken{}, app.Fail(ctx, errNotSetUp)
	}
	token, err := s.player.BrowserToken(ctx)
	return token, app.Fail(ctx, err)
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

// SetOffset shifts the song's lyrics by offsetMs, yours alone, to line them up with Spotify's track.
func (s *Service) SetOffset(ctx context.Context, songID, offsetMs int64) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.SetOffset(ctx, songID, offsetMs))
}

// ResetOffset goes back to the lyrics' own timing.
func (s *Service) ResetOffset(ctx context.Context, songID int64) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.player.ResetOffset(ctx, songID))
}
