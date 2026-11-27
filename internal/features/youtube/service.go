package youtube

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// Service is the bound Wails service for YouTube. Every exported method here is callable by the frontend. Off (not
// server mode), Status says so and the rest refuse.
type Service struct {
	finder  *Finder
	enabled bool
}

var errNotSetUp = fault.Invalid("youtube", "isn't set up on this server")

// NewService returns the service over repo. search is nil when the server has no YouTube API key.
func NewService(repo Repository, search Search, enabled bool) *Service {
	return &Service{finder: NewFinder(repo, search, time.Now), enabled: enabled}
}

// Status says whether YouTube can be used here, and whether it can search for videos.
func (s *Service) Status(context.Context) (Status, error) {
	if !s.enabled {
		return Status{}, nil
	}
	return Status{Enabled: true, Searchable: s.finder.Searchable()}, nil
}

// Video is your video for the song: the one you chose or search found before, or else one search finds now. Its ID
// is empty when there's none.
func (s *Service) Video(ctx context.Context, songID int64) (Video, error) {
	if !s.enabled {
		return Video{}, app.Fail(ctx, errNotSetUp)
	}
	video, err := s.finder.Video(ctx, songID)
	return video, app.Fail(ctx, err)
}

// SetVideo makes the video at link, a YouTube or YouTube Music link, your video for the song.
func (s *Service) SetVideo(ctx context.Context, songID int64, link string) (Video, error) {
	if !s.enabled {
		return Video{}, app.Fail(ctx, errNotSetUp)
	}
	video, err := s.finder.SetVideo(ctx, songID, link)
	return video, app.Fail(ctx, err)
}

// ForgetVideo drops your video for the song, so the next look searches again.
func (s *Service) ForgetVideo(ctx context.Context, songID int64) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.finder.ForgetVideo(ctx, songID))
}

// SetOffset shifts the song's lyrics by offsetMs, yours alone, to line them up with your video.
func (s *Service) SetOffset(ctx context.Context, songID, offsetMs int64) error {
	if !s.enabled {
		return app.Fail(ctx, errNotSetUp)
	}
	return app.Fail(ctx, s.finder.SetOffset(ctx, songID, offsetMs))
}
