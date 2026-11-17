package songs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// LRCLIBURL is the public LRCLIB API: free, with no key. Its documentation is at https://lrclib.net/docs.
const LRCLIBURL = "https://lrclib.net"

const (
	lrclibTimeout  = 10 * time.Second
	lrclibMaxBytes = 4 << 20
)

// LRCLIB is the LyricsSource for lrclib.net, a free, community-run lyrics database with synced lyrics.
type LRCLIB struct {
	base      string
	userAgent string
	client    *http.Client
}

// NewLRCLIB returns the LRCLIB source at base (LRCLIBURL, or a test server). LRCLIB asks clients to name themselves
// in the User-Agent, and allows identified clients more requests.
func NewLRCLIB(base, version string) *LRCLIB {
	return &LRCLIB{
		base:      strings.TrimRight(base, "/"),
		userAgent: "Utanabi " + version + " (https://github.com/JasonPKnoll/utanabi)",
		client:    &http.Client{Timeout: lrclibTimeout},
	}
}

// Code returns the lyric source code stored with songs from LRCLIB.
func (l *LRCLIB) Code() string { return "lrclib" }

// Name returns LRCLIB's name, as shown in the app.
func (l *LRCLIB) Name() string { return "LRCLIB" }

// lrclibRecord is one record from /api/search or /api/get/{id}. Duration is in seconds.
type lrclibRecord struct {
	ID           int64   `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  *string `json:"plainLyrics"`
	SyncedLyrics *string `json:"syncedLyrics"`
}

// Search returns up to 20 songs matching query that have lyrics. Instrumental tracks are left out.
func (l *LRCLIB) Search(ctx context.Context, query string) ([]Match, error) {
	var records []lrclibRecord
	if err := l.get(ctx, "/api/search?"+url.Values{"q": {query}}.Encode(), &records); err != nil {
		return nil, err
	}
	matches := make([]Match, 0, len(records))
	for _, r := range records {
		if !r.hasLyrics() {
			continue
		}
		matches = append(matches, l.match(r))
	}
	return matches, nil
}

// Lyrics returns one song with its lyrics, by its LRCLIB ID.
func (l *LRCLIB) Lyrics(ctx context.Context, ref string) (Fetched, error) {
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil || id <= 0 {
		return Fetched{}, fault.Invalid("ref", "must be an LRCLIB song ID")
	}
	var r lrclibRecord
	if err := l.get(ctx, "/api/get/"+strconv.FormatInt(id, 10), &r); err != nil {
		return Fetched{}, err
	}
	if !r.hasLyrics() {
		return Fetched{}, fault.Invalid("lyrics", "aren't available for this song")
	}
	return Fetched{Match: l.match(r), Plain: deref(r.PlainLyrics), Synced: deref(r.SyncedLyrics)}, nil
}

func (l *LRCLIB) match(r lrclibRecord) Match {
	return Match{
		Source:     l.Code(),
		SourceName: l.Name(),
		SourceRef:  strconv.FormatInt(r.ID, 10),
		Title:      r.TrackName,
		Artist:     r.ArtistName,
		Album:      r.AlbumName,
		DurationMS: int64(r.Duration * 1000),
		Synced:     strings.TrimSpace(deref(r.SyncedLyrics)) != "",
	}
}

func (r lrclibRecord) hasLyrics() bool {
	return !r.Instrumental &&
		(strings.TrimSpace(deref(r.PlainLyrics)) != "" || strings.TrimSpace(deref(r.SyncedLyrics)) != "")
}

// get fetches one JSON response and decodes it into out, turning network and HTTP failures into the project's errors.
func (l *LRCLIB) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.base+path, nil)
	if err != nil {
		return fmt.Errorf("songs: build lrclib request failed (%v)", err)
	}
	req.Header.Set("User-Agent", l.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		var netErr net.Error
		switch {
		case errors.Is(err, context.Canceled):
			return fault.ErrCanceled
		case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
			return fmt.Errorf("songs: lrclib took too long (%v); %w", err, fault.ErrTimeout)
		default:
			return fmt.Errorf("songs: lrclib unreachable (%v)", err)
		}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fault.ErrNotFound
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return fmt.Errorf("songs: lrclib is busy (status %d); %w", resp.StatusCode, fault.ErrTimeout)
	default:
		return fmt.Errorf("songs: lrclib request failed (status %d)", resp.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, lrclibMaxBytes)).Decode(out); err != nil {
		return fmt.Errorf("songs: lrclib sent an unexpected response (%v)", err)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
