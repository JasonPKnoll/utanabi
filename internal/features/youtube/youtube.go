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
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
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
// on the server. Without one, a video is pasted as a link. KeyFrom says where the key is: "settings" for one an admin
// entered, "server" for UTANABI_YOUTUBE_API_KEY, or "" for none. CanSetKey is true for an admin.
type Status struct {
	Enabled    bool   `json:"enabled"`
	Searchable bool   `json:"searchable"`
	KeyFrom    string `json:"keyFrom"`
	CanSetKey  bool   `json:"canSetKey"`
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

// Search is YouTube's search, with an API key.
type Search interface {
	// Videos returns the videos YouTube finds for query, most relevant first, with their lengths.
	Videos(ctx context.Context, key redact.Secret, query string) ([]Video, error)
	// Video returns one video's title, channel, and length, or fault.ErrNotFound.
	Video(ctx context.Context, key redact.Secret, id string) (Video, error)
}

// Keys keeps the API key an admin enters, sealed, for the whole server.
type Keys interface {
	// APIKey returns the key, or fault.ErrNotFound when there's none.
	APIKey(ctx context.Context) (redact.Secret, error)
	SaveAPIKey(ctx context.Context, key redact.Secret, now time.Time) error
	DeleteAPIKey(ctx context.Context) error
}

var (
	// ErrQuota means YouTube's search has run out for the day: the API key's quota is spent.
	ErrQuota = errors.New("youtube: quota exceeded")
	// ErrKeyRejected means Google turned the API key down: it isn't a key, or it was deleted or restricted.
	ErrKeyRejected = errors.New("youtube: API key rejected")
	// ErrAPIOff means the key's Google Cloud project hasn't switched on the YouTube Data API.
	ErrAPIOff = errors.New("youtube: YouTube Data API not enabled")
)

// Where the key comes from, as Status says.
const (
	KeyFromSettings = "settings"
	KeyFromServer   = "server"
)

// checkVideo is a video every key can read: YouTube's first, "Me at the zoo". Reading it costs 1 of the key's
// 10,000 daily units.
const checkVideo = "jNQXAC9IVRw"

var apiKeyShape = regexp.MustCompile(`^[0-9A-Za-z_-]{30,100}$`)

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
	errKeyBroken = fault.Invalid("youtube", "search's key was turned down; an admin can replace it in Settings")
)

// Finder finds and keeps each owner's video for their songs. It searches with the key an admin entered, or else the
// server's own (serverKey, from UTANABI_YOUTUBE_API_KEY), and without either, videos are pasted as links.
type Finder struct {
	repo      Repository
	keys      Keys
	serverKey redact.Secret
	search    Search
	now       func() time.Time
}

func NewFinder(repo Repository, keys Keys, serverKey redact.Secret, search Search, now func() time.Time) *Finder {
	return &Finder{repo: repo, keys: keys, serverKey: serverKey, search: search, now: now}
}

// key returns the key to search with and where it's from, or "" when there's none.
func (f *Finder) key(ctx context.Context) (redact.Secret, string, error) {
	key, err := f.keys.APIKey(ctx)
	if err == nil {
		return key, KeyFromSettings, nil
	}
	if !errors.Is(err, fault.ErrNotFound) {
		return redact.Secret{}, "", err
	}
	if !f.serverKey.IsZero() {
		return f.serverKey, KeyFromServer, nil
	}
	return redact.Secret{}, "", nil
}

// Status says whether the finder can search YouTube, with which key, and whether the caller may change it.
func (f *Finder) Status(ctx context.Context) (Status, error) {
	_, from, err := f.key(ctx)
	if err != nil {
		return Status{}, err
	}
	u, _ := identity.From(ctx)
	return Status{Enabled: true, Searchable: from != "", KeyFrom: from, CanSetKey: u.Admin}, nil
}

// SetAPIKey checks key with Google, then keeps it, sealed, as the key everyone's searches use. Admins only.
func (f *Finder) SetAPIKey(ctx context.Context, key string) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	return SaveKey(ctx, f.keys, f.search, key, f.now())
}

// ClearAPIKey forgets the key an admin entered, leaving the server's own, if it has one. Admins only.
func (f *Finder) ClearAPIKey(ctx context.Context) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	return f.keys.DeleteAPIKey(ctx)
}

// SaveKey checks key with Google by reading one video, then saves it in keys. A key out of quota for today is still
// a key, and is kept. The admin command saves a key through it too.
func SaveKey(ctx context.Context, keys Keys, search Search, key string, now time.Time) error {
	key = strings.TrimSpace(key)
	if !apiKeyShape.MatchString(key) {
		return fault.Invalid("key", "isn't a Google API key; it starts with AIza")
	}
	secret := redact.New(key)
	_, err := search.Video(ctx, secret, checkVideo)
	switch {
	case errors.Is(err, ErrKeyRejected):
		return fault.Invalid("key", "was turned down by Google; check that it was copied whole and isn't deleted")
	case errors.Is(err, ErrAPIOff):
		return fault.Invalid("key", "can't use YouTube yet; switch on the YouTube Data API v3 in its Google Cloud project")
	case err != nil && !errors.Is(err, ErrQuota) && !errors.Is(err, fault.ErrNotFound):
		return err
	}
	return keys.SaveAPIKey(ctx, secret, now)
}

func requireAdmin(ctx context.Context) error {
	u, ok := identity.From(ctx)
	if !ok {
		return fault.ErrUnauthorized
	}
	if !u.Admin {
		return fault.ErrForbidden
	}
	return nil
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
	key, from, err := f.key(ctx)
	if err != nil || from == "" {
		return Video{}, err
	}
	found, err := f.search.Videos(ctx, key, strings.TrimSpace(song.Artist+" "+song.Title))
	switch {
	case errors.Is(err, ErrQuota):
		return Video{}, errQuotaOver
	case errors.Is(err, ErrKeyRejected), errors.Is(err, ErrAPIOff):
		return Video{}, errKeyBroken
	case err != nil:
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
	key, from, err := f.key(ctx)
	if err != nil {
		return Video{}, err
	}
	if from != "" {
		// What it's called and how long it is, when search can say; a link it can't look up is still the video.
		found, err := f.search.Video(ctx, key, id)
		switch {
		case err == nil:
			video = found
		case errors.Is(err, fault.ErrNotFound), errors.Is(err, ErrQuota), errors.Is(err, ErrKeyRejected),
			errors.Is(err, ErrAPIOff):
		default:
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
