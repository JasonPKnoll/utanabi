// Package youtube plays a song's YouTube video in the page, with YouTube's embedded player, so the lyrics can follow
// it exactly. Each owner picks the video for their own songs: found by searching YouTube (with an API key), or
// pasted as a link.
package youtube

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// Video is a song's YouTube video. Chosen is true for one the owner pasted, false for one search found. OffsetMS is
// how far the owner has shifted the lyrics to line up with it: a line timed at t is sung at t + OffsetMS.
// DurationMS, Title, and Channel are empty for a pasted video when there's no search to ask about it.
type Video struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Channel    string `json:"channel"`
	DurationMS int64  `json:"durationMs"`
	OffsetMS   int64  `json:"offsetMs"`
	Chosen     bool   `json:"chosen"`
}

// Status says whether YouTube can be used here, and whether it can search for a song's video, which needs an API key
// on the server. Without one, a video is pasted as a link.
type Status struct {
	Enabled    bool `json:"enabled"`
	Searchable bool `json:"searchable"`
}

// Song is what finding a song's video needs: its title, artist, and length (0 when unknown).
type Song struct {
	Title      string
	Artist     string
	DurationMS int64
}

// Repository keeps each owner's video for their songs.
type Repository interface {
	// Song returns a song in the owner's library, or fault.ErrNotFound.
	Song(ctx context.Context, songID int64) (Song, error)
	// VideoFor returns the owner's video for a song, or fault.ErrNotFound.
	VideoFor(ctx context.Context, songID int64) (Video, error)
	SaveVideo(ctx context.Context, songID int64, video Video, now time.Time) error
	// SaveOffset sets the offset of the owner's video for a song, or returns fault.ErrNotFound when there's none.
	SaveOffset(ctx context.Context, songID, offsetMS int64) error
	DeleteVideo(ctx context.Context, songID int64) error
}

// Search is YouTube's search, behind its API key.
type Search interface {
	// Videos returns the videos YouTube finds for query, most relevant first, with their lengths.
	Videos(ctx context.Context, query string) ([]Video, error)
	// Video returns one video's title, channel, and length, or fault.ErrNotFound.
	Video(ctx context.Context, id string) (Video, error)
}

// ErrQuota means YouTube's search has run out for the day: the API key's quota is spent.
var ErrQuota = errors.New("youtube: quota exceeded")

const (
	// closeMS is how near a video's length must be to the lyrics' for search to take it over the most relevant:
	// lyrics timed to a song's official video are the video's length, to the second.
	closeMS     = 3000
	maxOffsetMS = 60000
	maxLinkLen  = 500
)

var (
	videoID      = regexp.MustCompile(`^[0-9A-Za-z_-]{11}$`)
	errBadLink   = fault.Invalid("link", "isn't a YouTube video link")
	errQuotaOver = fault.Invalid("youtube", "search has run out for today; paste a link to the video instead")
)

// Finder finds and keeps each owner's video for their songs. search is nil when the server has no API key.
type Finder struct {
	repo   Repository
	search Search
	now    func() time.Time
}

func NewFinder(repo Repository, search Search, now func() time.Time) *Finder {
	return &Finder{repo: repo, search: search, now: now}
}

// Searchable reports whether the finder can search YouTube.
func (f *Finder) Searchable() bool {
	return f.search != nil
}

// Video returns the caller's video for song songID: the one kept, or else the one search finds, kept from then on.
// Its ID is empty when there's none and no search to find one.
func (f *Finder) Video(ctx context.Context, songID int64) (Video, error) {
	if songID <= 0 {
		return Video{}, fault.Invalid("songId", "must be positive")
	}
	song, err := f.repo.Song(ctx, songID)
	if err != nil {
		return Video{}, err
	}
	video, err := f.repo.VideoFor(ctx, songID)
	if err == nil || !errors.Is(err, fault.ErrNotFound) {
		return video, err
	}
	if f.search == nil {
		return Video{}, nil
	}
	found, err := f.search.Videos(ctx, strings.TrimSpace(song.Artist+" "+song.Title))
	if errors.Is(err, ErrQuota) {
		return Video{}, errQuotaOver
	}
	if err != nil {
		return Video{}, err
	}
	video, ok := bestMatch(found, song.DurationMS)
	if !ok {
		return Video{}, nil
	}
	return video, f.repo.SaveVideo(ctx, songID, video, f.now())
}

// bestMatch picks the video for lyrics lyricsMS long: the closest in length within a few seconds, as the official
// video is for lyrics timed to it, or else the most relevant.
func bestMatch(videos []Video, lyricsMS int64) (Video, bool) {
	if len(videos) == 0 {
		return Video{}, false
	}
	best, bestOff := videos[0], int64(-1)
	for _, v := range videos {
		if lyricsMS <= 0 || v.DurationMS <= 0 {
			continue
		}
		off := abs(v.DurationMS - lyricsMS)
		if off <= closeMS && (bestOff < 0 || off < bestOff) {
			best, bestOff = v, off
		}
	}
	return best, true
}

// SetVideo makes the video at link the caller's video for song songID, in place of any before, with no offset.
func (f *Finder) SetVideo(ctx context.Context, songID int64, link string) (Video, error) {
	if songID <= 0 {
		return Video{}, fault.Invalid("songId", "must be positive")
	}
	id, err := ParseLink(link)
	if err != nil {
		return Video{}, err
	}
	if _, err := f.repo.Song(ctx, songID); err != nil {
		return Video{}, err
	}
	video := Video{ID: id}
	if f.search != nil {
		// What it's called and how long it is, when search can say; a link it can't look up is still the video.
		if found, err := f.search.Video(ctx, id); err == nil {
			video = found
		} else if !errors.Is(err, fault.ErrNotFound) && !errors.Is(err, ErrQuota) {
			return Video{}, err
		}
	}
	video.ID, video.Chosen, video.OffsetMS = id, true, 0
	return video, f.repo.SaveVideo(ctx, songID, video, f.now())
}

// ForgetVideo drops the caller's video for song songID, so the next look finds one again.
func (f *Finder) ForgetVideo(ctx context.Context, songID int64) error {
	if songID <= 0 {
		return fault.Invalid("songId", "must be positive")
	}
	return f.repo.DeleteVideo(ctx, songID)
}

// SetOffset shifts the song's lyrics by offsetMS for the caller, to line them up with their video.
func (f *Finder) SetOffset(ctx context.Context, songID, offsetMS int64) error {
	if songID <= 0 {
		return fault.Invalid("songId", "must be positive")
	}
	if abs(offsetMS) > maxOffsetMS {
		return fault.Invalid("offsetMs", "must be within a minute")
	}
	return f.repo.SaveOffset(ctx, songID, offsetMS)
}

// ParseLink reads a YouTube video's ID from a link to it, as YouTube, YouTube Music, and their share buttons write
// them (youtube.com/watch?v=…, youtu.be/…, music.youtube.com/watch?v=…, /shorts/…, /embed/…, /live/…), or from the
// ID alone.
func ParseLink(link string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" || len(link) > maxLinkLen {
		return "", errBadLink
	}
	if videoID.MatchString(link) {
		return link, nil
	}
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errBadLink
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
			break
		}
		for _, prefix := range []string{"/shorts/", "/embed/", "/live/", "/v/"} {
			if rest, ok := strings.CutPrefix(u.Path, prefix); ok {
				id = strings.Trim(rest, "/")
			}
		}
	}
	if !videoID.MatchString(id) {
		return "", errBadLink
	}
	return id, nil
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
