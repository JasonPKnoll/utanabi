// Package spotify lets a song follow along with Spotify: you play the song on your Spotify (on your phone, or any of
// your devices), and the app reads how far in it is, so the sung line moves with it. It can ask Spotify to play the
// song, pause, and jump to a line too. It never touches the audio, which stays in Spotify. Your Spotify account is
// connected once, through Spotify's own sign-in, and its tokens stay on the server.
package spotify

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

// Scopes are what the app asks Spotify for: reading what's playing, and playing, pausing, and seeking; and, for the
// page itself to play (Spotify's Web Playback SDK), streaming and reading the account's email and plan.
const Scopes = "user-read-playback-state user-modify-playback-state streaming user-read-email user-read-private"

// deviceID matches a Spotify device ID, such as the one the page's own player gets.
var deviceID = regexp.MustCompile(`^[0-9A-Za-z]{1,64}$`)

// Link is an owner's connection to Spotify: the tokens to call its API for them, and the name on their account.
type Link struct {
	AccessToken  redact.Secret
	RefreshToken redact.Secret
	ExpiresAt    time.Time
	DisplayName  string
	ConnectedAt  time.Time
}

// Tokens are what Spotify gives for a sign-in or a refresh. Refresh is empty when Spotify keeps the old one.
type Tokens struct {
	Access    redact.Secret
	Refresh   redact.Secret
	ExpiresIn time.Duration
}

// Song is what finding a song on Spotify needs: its title, artist, and length (0 when unknown).
type Song struct {
	Title      string
	Artist     string
	DurationMS int64
}

// Track is a track on Spotify.
type Track struct {
	ID         string
	Name       string
	Artists    []string
	DurationMS int64
}

// Device is one of the owner's Spotify devices, such as the Spotify app on their phone.
type Device struct {
	ID     string
	Name   string
	Type   string
	Active bool
}

// State is what's playing on the owner's Spotify. Track is zero for something that isn't a song, such as a podcast.
type State struct {
	Playing    bool
	ProgressMS int64
	Track      Track
	Device     Device
}

var (
	// ErrRejected means Spotify turned the token down: it ran out, or the owner took the app's access away.
	ErrRejected = errors.New("spotify: token rejected")
	// ErrNoDevice means none of the owner's devices is there to play on: Spotify isn't open anywhere.
	ErrNoDevice = errors.New("spotify: no device to play on")
	// ErrPremium means playing needs Spotify Premium.
	ErrPremium = errors.New("spotify: premium required")
	// ErrBusy means Spotify asked the app to slow down.
	ErrBusy = errors.New("spotify: too many requests")
)

// Romanizer writes Japanese text in romaji, so a title or a name written in one script matches it in another:
// Spotify lists 私生活, where the lyrics say Shiseikatsu.
type Romanizer interface {
	Romaji(ctx context.Context, text string) (string, error)
}

// API is Spotify's Web API, for one owner's tokens.
type API interface {
	// Exchange trades the code from Spotify's sign-in, and the PKCE verifier that goes with it, for tokens.
	Exchange(ctx context.Context, code, verifier, redirectURI string) (Tokens, error)
	// Refresh gets a new access token, or ErrRejected when the refresh token no longer works.
	Refresh(ctx context.Context, refresh redact.Secret) (Tokens, error)
	DisplayName(ctx context.Context, access redact.Secret) (string, error)
	// PlaybackState returns what's playing, and false when nothing is, paused or not.
	PlaybackState(ctx context.Context, access redact.Secret) (State, bool, error)
	Devices(ctx context.Context, access redact.Secret) ([]Device, error)
	SearchTracks(ctx context.Context, access redact.Secret, query string) ([]Track, error)
	// Play plays a track from positionMS on a device, or returns ErrNoDevice.
	Play(ctx context.Context, access redact.Secret, deviceID, trackID string, positionMS int64) error
	Resume(ctx context.Context, access redact.Secret) error
	Pause(ctx context.Context, access redact.Secret) error
	Seek(ctx context.Context, access redact.Secret, positionMS int64) error
}

// Repository keeps each owner's link, and the Spotify track of each song.
type Repository interface {
	// Link returns the owner's link, or fault.ErrNotFound when they haven't connected Spotify.
	Link(ctx context.Context) (Link, error)
	SaveLink(ctx context.Context, link Link) error
	DeleteLink(ctx context.Context) error
	// Song returns a song in the owner's library, or fault.ErrNotFound.
	Song(ctx context.Context, songID int64) (Song, error)
	// TrackFor returns the Spotify track found for a song in the owner's library, or fault.ErrNotFound. Its
	// DurationMS is 0 when not known yet.
	TrackFor(ctx context.Context, songID int64) (Track, error)
	SaveTrack(ctx context.Context, songID int64, track Track, now time.Time) error
	// Offset returns how far the owner has shifted a song's lyrics, or fault.ErrNotFound when they haven't.
	Offset(ctx context.Context, songID int64) (int64, error)
	SaveOffset(ctx context.Context, songID, offsetMS int64) error
	DeleteOffset(ctx context.Context, songID int64) error
}

// Status is whether Spotify can be used here, and whether the caller has connected theirs.
type Status struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Name      string `json:"name"`
}

// Playback is what's playing, as a song's view needs it. TrackURI is the song's track on Spotify (spotify:track:…),
// empty when Spotify doesn't have it. Active is true while something plays or is paused; Matches when that's this
// song, so the lyrics can follow it. Ready is true when a device is there to play on, so Play can start without
// opening Spotify.
type Playback struct {
	TrackURI   string `json:"trackUri"`
	Active     bool   `json:"active"`
	Playing    bool   `json:"playing"`
	Matches    bool   `json:"matches"`
	ProgressMS int64  `json:"progressMs"`
	Device     string `json:"device"`
	Ready      bool   `json:"ready"`
	// OffsetMS is how far you've shifted the lyrics to line up with the track (OffsetSet): a line timed at t is sung
	// at t + OffsetMS. 0 until you set one.
	OffsetMS  int64 `json:"offsetMs"`
	OffsetSet bool  `json:"offsetSet"`
	// TimedForMS is the length the lyrics are timed for, when it's off the track's and you haven't set an offset, so
	// they may not line up; 0 otherwise.
	TimedForMS int64 `json:"timedForMs"`
}

// PlayResult says whether the song started. When it didn't, no device was there to play on: opening TrackURI opens
// the song in the Spotify app, which wakes it.
type PlayResult struct {
	Started  bool   `json:"started"`
	TrackURI string `json:"trackUri"`
}

var (
	errNotConnected = fault.Invalid("spotify", "isn't connected; connect it in Settings")
	errDisconnected = fault.Invalid("spotify", "was disconnected; connect it again in Settings")
	errNotOnSpotify = fault.Invalid("song", "isn't on Spotify")
	errNeedsPremium = fault.Invalid("spotify", "needs Spotify Premium to play songs")
	errSlowDown     = fault.Invalid("spotify", "is busy; please try again in a moment")
)

// Connect finishes Spotify's sign-in for the caller: it trades the code for tokens and saves the link.
func Connect(ctx context.Context, repo Repository, api API, code, verifier, redirectURI string, now time.Time) error {
	if code == "" {
		return fault.Invalid("code", "is missing")
	}
	tokens, err := api.Exchange(ctx, code, verifier, redirectURI)
	if err != nil {
		return err
	}
	name, err := api.DisplayName(ctx, tokens.Access)
	if err != nil {
		return err
	}
	return repo.SaveLink(ctx, Link{AccessToken: tokens.Access, RefreshToken: tokens.Refresh,
		ExpiresAt: now.Add(tokens.ExpiresIn), DisplayName: name, ConnectedAt: now})
}

// Player is the caller's Spotify: what's playing, and playing, pausing, and seeking.
type Player struct {
	repo   Repository
	api    API
	romaji Romanizer
	now    func() time.Time
}

func NewPlayer(repo Repository, api API, romaji Romanizer, now func() time.Time) *Player {
	return &Player{repo: repo, api: api, romaji: romaji, now: now}
}

// Status says whether the caller has connected Spotify.
func (p *Player) Status(ctx context.Context) (Status, error) {
	link, err := p.repo.Link(ctx)
	if errors.Is(err, fault.ErrNotFound) {
		return Status{Enabled: true}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Enabled: true, Connected: true, Name: link.DisplayName}, nil
}

// Disconnect forgets the caller's Spotify tokens.
func (p *Player) Disconnect(ctx context.Context) error {
	return p.repo.DeleteLink(ctx)
}

// NowPlaying says what's playing, and whether it's the song songID, so its lyrics can follow along.
func (p *Player) NowPlaying(ctx context.Context, songID int64) (Playback, error) {
	if songID <= 0 {
		return Playback{}, fault.Invalid("songId", "must be positive")
	}
	song, err := p.repo.Song(ctx, songID)
	if err != nil {
		return Playback{}, err
	}
	var out Playback
	err = p.withToken(ctx, func(access redact.Secret) error {
		track, err := p.track(ctx, access, songID, song)
		if err != nil {
			return err
		}
		out = Playback{TrackURI: uri(track.ID)}
		state, active, err := p.api.PlaybackState(ctx, access)
		if err != nil {
			return err
		}
		if !active {
			devices, err := p.api.Devices(ctx, access)
			out.Ready = len(devices) > 0
			if err != nil {
				return err
			}
			return p.lineUp(ctx, &out, songID, song, track.DurationMS)
		}
		out.Active, out.Ready, out.Playing = true, true, state.Playing
		out.ProgressMS, out.Device = state.ProgressMS, state.Device.Name
		switch {
		case state.Track.ID == "":
		case state.Track.ID == track.ID:
			out.Matches = true
			if track.DurationMS == 0 && state.Track.DurationMS > 0 {
				// Matched before tracks kept their length: keep it now, for lining up before it plays.
				if err := p.repo.SaveTrack(ctx, songID, state.Track, p.now()); err != nil {
					return err
				}
			}
			track.DurationMS = state.Track.DurationMS
		case track.ID == "":
			// Played from Spotify itself before the app found the song there: when it's the song, remember it.
			same, err := p.sameSong(ctx, state.Track, song)
			if err != nil || !same {
				return err
			}
			out.Matches, out.TrackURI, track = true, uri(state.Track.ID), state.Track
			if err := p.repo.SaveTrack(ctx, songID, state.Track, p.now()); err != nil {
				return err
			}
		}
		return p.lineUp(ctx, &out, songID, song, track.DurationMS)
	})
	return out, err
}

// lineUp says how far the song's lyrics are shifted to line up with its track, trackMS long: by the offset the owner
// set, or not at all. Without one, while the song plays, it says how long the lyrics are timed for when that's off
// the track's length, so the bar can suggest setting the timing.
func (p *Player) lineUp(ctx context.Context, out *Playback, songID int64, song Song, trackMS int64) error {
	offset, err := p.repo.Offset(ctx, songID)
	if err == nil {
		out.OffsetMS, out.OffsetSet = offset, true
		return nil
	}
	if !errors.Is(err, fault.ErrNotFound) {
		return err
	}
	if out.Matches {
		out.TimedForMS = otherLength(song.DurationMS, trackMS)
	}
	return nil
}

// SetOffset shifts the song's lyrics by offsetMS for the caller, to line them up with its track.
func (p *Player) SetOffset(ctx context.Context, songID, offsetMS int64) error {
	if songID <= 0 {
		return fault.Invalid("songId", "must be positive")
	}
	if abs(offsetMS) > maxOffsetMS {
		return fault.Invalid("offsetMs", "must be within a minute")
	}
	if _, err := p.repo.Song(ctx, songID); err != nil {
		return err
	}
	return p.repo.SaveOffset(ctx, songID, offsetMS)
}

// ResetOffset goes back to the lyrics' own timing.
func (p *Player) ResetOffset(ctx context.Context, songID int64) error {
	if songID <= 0 {
		return fault.Invalid("songId", "must be positive")
	}
	return p.repo.DeleteOffset(ctx, songID)
}

// Play plays the song songID on the caller's Spotify from positionMS: on the device onDevice, such as the page's own
// player, or with none given, on the one the caller is most likely listening on. With no device to play on, it
// doesn't start, and says so.
func (p *Player) Play(ctx context.Context, songID, positionMS int64, onDevice string) (PlayResult, error) {
	if songID <= 0 {
		return PlayResult{}, fault.Invalid("songId", "must be positive")
	}
	if positionMS < 0 {
		return PlayResult{}, fault.Invalid("positionMs", "must not be negative")
	}
	if onDevice != "" && !deviceID.MatchString(onDevice) {
		return PlayResult{}, fault.Invalid("deviceId", "isn't a Spotify device ID")
	}
	song, err := p.repo.Song(ctx, songID)
	if err != nil {
		return PlayResult{}, err
	}
	var out PlayResult
	err = p.withToken(ctx, func(access redact.Secret) error {
		track, err := p.track(ctx, access, songID, song)
		if err != nil {
			return err
		}
		if track.ID == "" {
			return errNotOnSpotify
		}
		out = PlayResult{TrackURI: uri(track.ID)}
		device := Device{ID: onDevice}
		if onDevice == "" {
			devices, err := p.api.Devices(ctx, access)
			if err != nil {
				return err
			}
			var ok bool
			if device, ok = pick(devices); !ok {
				return nil
			}
		}
		err = p.api.Play(ctx, access, device.ID, track.ID, positionMS)
		if errors.Is(err, ErrNoDevice) {
			return nil
		}
		out.Started = err == nil
		return err
	})
	return out, err
}

// BrowserToken is an access token for the page's own player, which plays in the browser and needs one there. It's the
// caller's own, for their own page, and runs out within the hour; the refresh token never leaves the server.
type BrowserToken struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// BrowserToken returns a working access token for the caller's page to play with, refreshed first when it's close to
// running out.
func (p *Player) BrowserToken(ctx context.Context) (BrowserToken, error) {
	link, err := p.repo.Link(ctx)
	if errors.Is(err, fault.ErrNotFound) {
		return BrowserToken{}, errNotConnected
	}
	if err != nil {
		return BrowserToken{}, err
	}
	if !p.now().Before(link.ExpiresAt.Add(-5 * time.Minute)) {
		if link, err = p.refresh(ctx, link); err != nil {
			return BrowserToken{}, err
		}
	}
	// The value leaves the process here, to the owner's own page.
	return BrowserToken{AccessToken: link.AccessToken.Reveal(), ExpiresAt: link.ExpiresAt.UnixMilli()}, nil
}

// Resume carries on with what's paused.
func (p *Player) Resume(ctx context.Context) error {
	return p.withToken(ctx, func(access redact.Secret) error { return p.api.Resume(ctx, access) })
}

// Pause pauses what's playing.
func (p *Player) Pause(ctx context.Context) error {
	return p.withToken(ctx, func(access redact.Secret) error { return p.api.Pause(ctx, access) })
}

// Seek jumps what's playing to positionMS, such as the start of a line.
func (p *Player) Seek(ctx context.Context, positionMS int64) error {
	if positionMS < 0 {
		return fault.Invalid("positionMs", "must not be negative")
	}
	return p.withToken(ctx, func(access redact.Secret) error { return p.api.Seek(ctx, access, positionMS) })
}

// withToken calls fn with a working access token, refreshing it when it has run out or Spotify turns it down. When
// even the refresh token is turned down, the link is forgotten: the owner has to connect again.
func (p *Player) withToken(ctx context.Context, fn func(access redact.Secret) error) error {
	link, err := p.repo.Link(ctx)
	if errors.Is(err, fault.ErrNotFound) {
		return errNotConnected
	}
	if err != nil {
		return err
	}
	// A token close to running out is refreshed first, so a call doesn't fail halfway.
	if !p.now().Before(link.ExpiresAt.Add(-time.Minute)) {
		if link, err = p.refresh(ctx, link); err != nil {
			return err
		}
	}
	err = fn(link.AccessToken)
	if errors.Is(err, ErrRejected) {
		if link, err = p.refresh(ctx, link); err != nil {
			return err
		}
		err = fn(link.AccessToken)
	}
	return friendly(err)
}

func (p *Player) refresh(ctx context.Context, link Link) (Link, error) {
	tokens, err := p.api.Refresh(ctx, link.RefreshToken)
	if errors.Is(err, ErrRejected) {
		return Link{}, errors.Join(errDisconnected, p.repo.DeleteLink(ctx))
	}
	if err != nil {
		return Link{}, friendly(err)
	}
	link.AccessToken, link.ExpiresAt = tokens.Access, p.now().Add(tokens.ExpiresIn)
	if !tokens.Refresh.IsZero() {
		link.RefreshToken = tokens.Refresh
	}
	return link, p.repo.SaveLink(ctx, link)
}

// friendly turns Spotify's refusals into what the caller can do about them.
func friendly(err error) error {
	switch {
	case errors.Is(err, ErrRejected):
		return errDisconnected
	case errors.Is(err, ErrPremium):
		return errNeedsPremium
	case errors.Is(err, ErrBusy):
		return errSlowDown
	}
	return err
}

// track returns the song's Spotify track, finding it the first time by searching for its title and artist, and
// empty when Spotify doesn't seem to have it.
func (p *Player) track(ctx context.Context, access redact.Secret, songID int64, song Song) (Track, error) {
	track, err := p.repo.TrackFor(ctx, songID)
	if err == nil || !errors.Is(err, fault.ErrNotFound) {
		return track, err
	}
	tracks, err := p.api.SearchTracks(ctx, access, strings.TrimSpace(song.Title+" "+song.Artist))
	if err != nil {
		return Track{}, err
	}
	best, ok, err := p.bestMatch(ctx, tracks, song)
	if err != nil || !ok {
		return Track{}, err
	}
	return best, p.repo.SaveTrack(ctx, songID, best, p.now())
}

// pick chooses where to play: the device playing now, then a phone, then any.
func pick(devices []Device) (Device, bool) {
	for _, d := range devices {
		if d.Active {
			return d, true
		}
	}
	for _, d := range devices {
		if strings.EqualFold(d.Type, "smartphone") {
			return d, true
		}
	}
	if len(devices) > 0 {
		return devices[0], true
	}
	return Device{}, false
}

func uri(trackID string) string {
	if trackID == "" {
		return ""
	}
	return "spotify:track:" + trackID
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
