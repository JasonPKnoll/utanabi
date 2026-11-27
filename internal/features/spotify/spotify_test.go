package spotify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
)

var clock = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

const kazeTrack = "3sY7Sdc9a5xP0M2YbS0Xx7"

type fakeRepo struct {
	link    *Link
	deleted bool
	songs   map[int64]Song
	tracks  map[int64]Track
	offsets map[int64]int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		link: &Link{AccessToken: redact.New("access"), RefreshToken: redact.New("refresh"),
			ExpiresAt: clock.Add(time.Hour), DisplayName: "Hana", ConnectedAt: clock},
		songs:   map[int64]Song{1: {Title: "Fujii Kaze - Shinunoga E-Wa", Artist: "Fujii Kaze", DurationMS: 185000}},
		tracks:  map[int64]Track{},
		offsets: map[int64]int64{},
	}
}

func (f *fakeRepo) Link(context.Context) (Link, error) {
	if f.link == nil {
		return Link{}, fault.ErrNotFound
	}
	return *f.link, nil
}

func (f *fakeRepo) SaveLink(_ context.Context, link Link) error {
	f.link = &link
	return nil
}

func (f *fakeRepo) DeleteLink(context.Context) error {
	f.link, f.deleted = nil, true
	return nil
}

func (f *fakeRepo) Song(_ context.Context, id int64) (Song, error) {
	s, ok := f.songs[id]
	if !ok {
		return Song{}, fault.ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) TrackFor(_ context.Context, id int64) (Track, error) {
	t, ok := f.tracks[id]
	if !ok {
		return Track{}, fault.ErrNotFound
	}
	return t, nil
}

func (f *fakeRepo) SaveTrack(_ context.Context, id int64, track Track, _ time.Time) error {
	f.tracks[id] = Track{ID: track.ID, DurationMS: track.DurationMS}
	return nil
}

func (f *fakeRepo) Offset(_ context.Context, id int64) (int64, error) {
	o, ok := f.offsets[id]
	if !ok {
		return 0, fault.ErrNotFound
	}
	return o, nil
}

func (f *fakeRepo) SaveOffset(_ context.Context, id, offsetMS int64) error {
	f.offsets[id] = offsetMS
	return nil
}

func (f *fakeRepo) DeleteOffset(_ context.Context, id int64) error {
	delete(f.offsets, id)
	return nil
}

// fakeAPI is Spotify for one owner. It accepts only the access token it last handed out.
type fakeAPI struct {
	access     string
	refreshErr error
	refreshes  int
	newRefresh string
	state      *State
	devices    []Device
	found      []Track
	searches   int
	playErr    error
	played     []string
	seeked     []int64
	rejectOnce bool
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{access: "access", found: []Track{
		{ID: "0000000000000000000000", Name: "Shinunoga E-Wa (Live)", Artists: []string{"Fujii Kaze"}, DurationMS: 240000},
		{ID: kazeTrack, Name: "Shinunoga E-Wa", Artists: []string{"Fujii Kaze"}, DurationMS: 185500},
		{ID: "1111111111111111111111", Name: "Shinunoga E-Wa", Artists: []string{"Someone Else"}, DurationMS: 185000},
	}}
}

func (f *fakeAPI) check(access redact.Secret) error {
	if f.rejectOnce {
		f.rejectOnce = false
		return ErrRejected
	}
	if access.Reveal() != f.access {
		return ErrRejected
	}
	return nil
}

func (f *fakeAPI) Exchange(context.Context, string, string, string) (Tokens, error) {
	return Tokens{Access: redact.New(f.access), Refresh: redact.New("refresh"), ExpiresIn: time.Hour}, nil
}

func (f *fakeAPI) Refresh(_ context.Context, refresh redact.Secret) (Tokens, error) {
	f.refreshes++
	if f.refreshErr != nil {
		return Tokens{}, f.refreshErr
	}
	if refresh.Reveal() != "refresh" {
		return Tokens{}, ErrRejected
	}
	f.access = "access-2"
	return Tokens{Access: redact.New(f.access), Refresh: redact.New(f.newRefresh), ExpiresIn: time.Hour}, nil
}

func (f *fakeAPI) DisplayName(_ context.Context, access redact.Secret) (string, error) {
	return "Hana", f.check(access)
}

func (f *fakeAPI) PlaybackState(_ context.Context, access redact.Secret) (State, bool, error) {
	if err := f.check(access); err != nil {
		return State{}, false, err
	}
	if f.state == nil {
		return State{}, false, nil
	}
	return *f.state, true, nil
}

func (f *fakeAPI) Devices(_ context.Context, access redact.Secret) ([]Device, error) {
	return f.devices, f.check(access)
}

func (f *fakeAPI) SearchTracks(_ context.Context, access redact.Secret, _ string) ([]Track, error) {
	f.searches++
	return f.found, f.check(access)
}

func (f *fakeAPI) Play(_ context.Context, access redact.Secret, deviceID, trackID string, positionMS int64) error {
	if err := f.check(access); err != nil {
		return err
	}
	if f.playErr != nil {
		return f.playErr
	}
	f.played = append(f.played, deviceID+" "+trackID)
	f.seeked = append(f.seeked, positionMS)
	return nil
}

func (f *fakeAPI) Resume(_ context.Context, access redact.Secret) error { return f.check(access) }
func (f *fakeAPI) Pause(_ context.Context, access redact.Secret) error  { return f.check(access) }

func (f *fakeAPI) Seek(_ context.Context, access redact.Secret, positionMS int64) error {
	f.seeked = append(f.seeked, positionMS)
	return f.check(access)
}

// fakeRomaji reads the Japanese names the tests use, and leaves everything else as written.
type fakeRomaji struct{ err error }

var romajiOf = map[string]string{"私生活": "shiseikatsu", "東京事変": "toukyou jihen", "夜に駆ける": "yoru ni kakeru",
	"愛": "ai"}

func (f fakeRomaji) Romaji(_ context.Context, text string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	for ja, r := range romajiOf {
		text = strings.ReplaceAll(text, ja, r)
	}
	return text, nil
}

func newTestPlayer(repo *fakeRepo, api *fakeAPI) *Player {
	return NewPlayer(repo, api, fakeRomaji{}, func() time.Time { return clock })
}

func invalidField(err error) string {
	var invalid *fault.InvalidError
	if errors.As(err, &invalid) {
		return invalid.Field
	}
	return ""
}

func TestStatus(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	got, err := newTestPlayer(repo, newFakeAPI()).Status(t.Context())
	if err != nil || got != (Status{Enabled: true, Connected: true, Name: "Hana"}) {
		t.Errorf("connected: got %+v, %v", got, err)
	}
	repo.link = nil
	got, err = newTestPlayer(repo, newFakeAPI()).Status(t.Context())
	if err != nil || got != (Status{Enabled: true}) {
		t.Errorf("not connected: got %+v, %v", got, err)
	}
}

func TestConnectSavesTheLink(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.link = nil
	if err := Connect(t.Context(), repo, newFakeAPI(), "code", "verifier", "https://x/cb", clock); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if repo.link == nil || repo.link.DisplayName != "Hana" || !repo.link.ExpiresAt.Equal(clock.Add(time.Hour)) {
		t.Errorf("link = %+v", repo.link)
	}
	if err := Connect(t.Context(), repo, newFakeAPI(), "", "verifier", "https://x/cb", clock); invalidField(err) != "code" {
		t.Errorf("no code: %v, want invalid code", err)
	}
}

func TestATokenThatRunsOutIsRefreshed(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	repo.link.ExpiresAt = clock.Add(30 * time.Second)
	api.access, api.newRefresh = "", "refresh-2"
	if err := newTestPlayer(repo, api).Pause(t.Context()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if api.refreshes != 1 || repo.link.AccessToken.Reveal() != "access-2" || repo.link.RefreshToken.Reveal() != "refresh-2" {
		t.Errorf("after refresh: %d refreshes, link %s / %s", api.refreshes, repo.link.AccessToken.Reveal(),
			repo.link.RefreshToken.Reveal())
	}
}

func TestATurnedDownTokenIsRefreshedAndTheCallTriedAgain(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	api.rejectOnce = true
	if err := newTestPlayer(repo, api).Pause(t.Context()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if api.refreshes != 1 || repo.link.RefreshToken.Reveal() != "refresh" {
		t.Errorf("%d refreshes, refresh token %q; want one, and the old refresh token kept when no new one comes",
			api.refreshes, repo.link.RefreshToken.Reveal())
	}
}

func TestARefusedRefreshDisconnects(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	repo.link.ExpiresAt = clock
	api.refreshErr = ErrRejected
	err := newTestPlayer(repo, api).Pause(t.Context())
	if invalidField(err) != "spotify" || !repo.deleted {
		t.Errorf("err = %v, deleted %v; want disconnected and the link forgotten", err, repo.deleted)
	}
}

func TestNotConnected(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.link = nil
	if _, err := newTestPlayer(repo, newFakeAPI()).NowPlaying(t.Context(), 1); invalidField(err) != "spotify" {
		t.Errorf("err = %v, want invalid spotify", err)
	}
}

func TestNowPlaying(t *testing.T) {
	t.Parallel()

	shorter := int64(-3000)
	tests := []struct {
		name     string
		cached   string
		cachedMS int64
		offset   *int64
		state    *State
		phone    bool
		want     Playback
	}{
		{name: "nothing playing, Spotify open on the phone", phone: true,
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Ready: true}},
		{name: "nothing playing, and Spotify nowhere",
			want: Playback{TrackURI: "spotify:track:" + kazeTrack}},
		{name: "this song, playing", cached: kazeTrack,
			state: &State{Playing: true, ProgressMS: 61000, Track: Track{ID: kazeTrack}, Device: Device{Name: "iPhone"}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Matches: true,
				ProgressMS: 61000, Device: "iPhone", Ready: true}},
		{name: "another song, paused", cached: kazeTrack,
			state: &State{ProgressMS: 5000, Track: Track{ID: "2222222222222222222222", Name: "Matsuri",
				Artists: []string{"Fujii Kaze"}}, Device: Device{Name: "iPhone"}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, ProgressMS: 5000, Device: "iPhone",
				Ready: true}},
		{name: "this song, timed for a longer version", cached: kazeTrack,
			state: &State{Playing: true, ProgressMS: 1000, Track: Track{ID: kazeTrack, DurationMS: 160000}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Matches: true,
				ProgressMS: 1000, Ready: true, TimedForMS: 185000}},
		{name: "this song, half a second off", cached: kazeTrack,
			state: &State{Playing: true, ProgressMS: 1000, Track: Track{ID: kazeTrack, DurationMS: 185500}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Matches: true,
				ProgressMS: 1000, Ready: true}},
		{name: "this song, its track a few seconds shorter, says the lyrics may not line up", cached: kazeTrack,
			state: &State{Playing: true, ProgressMS: 1000, Track: Track{ID: kazeTrack, DurationMS: 179500}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Matches: true,
				ProgressMS: 1000, Ready: true, TimedForMS: 185000}},
		{name: "this song, with an offset you set", cached: kazeTrack, offset: &shorter,
			state: &State{Playing: true, ProgressMS: 1000, Track: Track{ID: kazeTrack, DurationMS: 160000}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Matches: true,
				ProgressMS: 1000, Ready: true, OffsetMS: -3000, OffsetSet: true}},
		{name: "nothing playing, with an offset you set", cached: kazeTrack, cachedMS: 190000, offset: &shorter,
			phone: true, want: Playback{TrackURI: "spotify:track:" + kazeTrack, Ready: true, OffsetMS: -3000,
				OffsetSet: true}},
		{name: "a podcast", cached: kazeTrack, state: &State{Playing: true, Device: Device{Name: "iPhone"}},
			want: Playback{TrackURI: "spotify:track:" + kazeTrack, Active: true, Playing: true, Device: "iPhone",
				Ready: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, api := newFakeRepo(), newFakeAPI()
			if tt.cached != "" {
				repo.tracks[1] = Track{ID: tt.cached, DurationMS: tt.cachedMS}
			}
			if tt.offset != nil {
				repo.offsets[1] = *tt.offset
			}
			api.state = tt.state
			if tt.phone {
				api.devices = []Device{{ID: "phone", Type: "Smartphone"}}
			}
			got, err := newTestPlayer(repo, api).NowPlaying(t.Context(), 1)
			if err != nil {
				t.Fatalf("NowPlaying: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNowPlayingFindsTheSongOnceAndRemembersIt(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	p := newTestPlayer(repo, api)
	for range 3 {
		if _, err := p.NowPlaying(t.Context(), 1); err != nil {
			t.Fatalf("NowPlaying: %v", err)
		}
	}
	if api.searches != 1 || repo.tracks[1].ID != kazeTrack || repo.tracks[1].DurationMS != 185500 {
		t.Errorf("%d searches, track %+v; want one search finding the studio version, with its length", api.searches,
			repo.tracks[1])
	}
}

func TestNowPlayingRecognisesTheSongPlayedFromSpotify(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	api.found = nil
	api.state = &State{Playing: true, ProgressMS: 1000,
		Track: Track{ID: kazeTrack, Name: "Shinunoga E-Wa", Artists: []string{"Fujii Kaze"}}}
	got, err := newTestPlayer(repo, api).NowPlaying(t.Context(), 1)
	if err != nil {
		t.Fatalf("NowPlaying: %v", err)
	}
	if !got.Matches || got.TrackURI != "spotify:track:"+kazeTrack || repo.tracks[1].ID != kazeTrack {
		t.Errorf("got %+v, saved %+v; want it matched and remembered", got, repo.tracks[1])
	}
}

func TestPlay(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	api.devices = []Device{{ID: "mac", Type: "Computer"}, {ID: "phone", Type: "Smartphone"}}
	got, err := newTestPlayer(repo, api).Play(t.Context(), 1, 42000)
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if !got.Started || len(api.played) != 1 || api.played[0] != "phone "+kazeTrack || api.seeked[0] != 42000 {
		t.Errorf("got %+v, played %v at %v; want the phone, from 42 s", got, api.played, api.seeked)
	}
}

func TestPlayPrefersTheDevicePlayingNow(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	api.devices = []Device{{ID: "phone", Type: "Smartphone"}, {ID: "speaker", Type: "Speaker", Active: true}}
	if _, err := newTestPlayer(repo, api).Play(t.Context(), 1, 0); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if api.played[0] != "speaker "+kazeTrack {
		t.Errorf("played on %v, want the active speaker", api.played)
	}
}

func TestPlayWithNoDeviceSaysToOpenSpotify(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		devices []Device
		err     error
	}{
		{name: "no devices"},
		{name: "a device that has gone", devices: []Device{{ID: "phone"}}, err: ErrNoDevice},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, api := newFakeRepo(), newFakeAPI()
			api.devices, api.playErr = tt.devices, tt.err
			got, err := newTestPlayer(repo, api).Play(t.Context(), 1, 0)
			if err != nil || got != (PlayResult{TrackURI: "spotify:track:" + kazeTrack}) {
				t.Errorf("got %+v, %v; want not started, with the track to open", got, err)
			}
		})
	}
}

func TestPlayRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  func(*fakeRepo, *fakeAPI)
		songID int64
		pos    int64
		field  string
	}{
		{name: "no song", songID: 0, field: "songId"},
		{name: "a negative position", songID: 1, pos: -1, field: "positionMs"},
		{name: "a song Spotify doesn't have", songID: 1, field: "song",
			setup: func(_ *fakeRepo, a *fakeAPI) { a.found = nil }},
		{name: "no Premium", songID: 1, field: "spotify",
			setup: func(_ *fakeRepo, a *fakeAPI) { a.devices, a.playErr = []Device{{ID: "phone"}}, ErrPremium }},
		{name: "too many requests", songID: 1, field: "spotify",
			setup: func(_ *fakeRepo, a *fakeAPI) { a.devices, a.playErr = []Device{{ID: "phone"}}, ErrBusy }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, api := newFakeRepo(), newFakeAPI()
			if tt.setup != nil {
				tt.setup(repo, api)
			}
			if _, err := newTestPlayer(repo, api).Play(t.Context(), tt.songID, tt.pos); invalidField(err) != tt.field {
				t.Errorf("err = %v, want invalid %s", err, tt.field)
			}
		})
	}
}

func TestPlayASongNotInTheLibrary(t *testing.T) {
	t.Parallel()

	if _, err := newTestPlayer(newFakeRepo(), newFakeAPI()).Play(t.Context(), 9, 0); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want not found", err)
	}
}

func TestSeek(t *testing.T) {
	t.Parallel()

	api := newFakeAPI()
	p := newTestPlayer(newFakeRepo(), api)
	if err := p.Seek(t.Context(), 30500); err != nil || len(api.seeked) != 1 || api.seeked[0] != 30500 {
		t.Errorf("Seek: %v, seeked %v", err, api.seeked)
	}
	if err := p.Seek(t.Context(), -5); invalidField(err) != "positionMs" {
		t.Errorf("negative: %v, want invalid positionMs", err)
	}
}

func TestBestMatch(t *testing.T) {
	t.Parallel()

	yoasobi := Song{Title: "夜に駆ける", Artist: "YOASOBI", DurationMS: 261000}
	jihen := Song{Title: "Shiseikatsu", Artist: "Tokyo Jihen", DurationMS: 229546}
	tests := []struct {
		name   string
		song   Song
		tracks []Track
		want   string
	}{
		{name: "the one by the artist with the title", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "夜に駆ける", Artists: []string{"Cover Band"}},
				{ID: "b", Name: "夜に駆ける", Artists: []string{"YOASOBI"}, DurationMS: 261500}}, want: "b"},
		{name: "the closest in length", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "夜に駆ける - Live", Artists: []string{"YOASOBI"}, DurationMS: 300000},
				{ID: "b", Name: "夜に駆ける", Artists: []string{"YOASOBI"}, DurationMS: 260800}}, want: "b"},
		{name: "full-width letters and case don't matter", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "夜に駆ける", Artists: []string{"ＹＯＡＳＯＢＩ"}}}, want: "a"},
		{name: "nothing by the artist", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "夜に駆ける", Artists: []string{"Someone"}}}},
		{name: "by the artist, but another song", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "群青", Artists: []string{"YOASOBI"}, DurationMS: 250000}}},
		{name: "the title over another song of the same length", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "群青", Artists: []string{"YOASOBI"}, DurationMS: 261000},
				{ID: "b", Name: "夜に駆ける", Artists: []string{"YOASOBI"}, DurationMS: 238000}}, want: "b"},
		{name: "the title with no length", song: yoasobi,
			tracks: []Track{{ID: "a", Name: "夜に駆ける", Artists: []string{"YOASOBI"}}}, want: "a"},
		{name: "a title in romaji matches it in kanji", song: Song{Title: "Yoru ni Kakeru", Artist: "YOASOBI"},
			tracks: []Track{{ID: "a", Name: "夜に駆ける", Artists: []string{"YOASOBI"}}}, want: "a"},
		{name: "an artist in romaji matches them in kanji, with the long vowels spelled another way",
			song:   Song{Title: "Shiseikatsu", Artist: "Tōkyō Jihen"},
			tracks: []Track{{ID: "a", Name: "私生活", Artists: []string{"東京事変"}}}, want: "a"},
		{name: "a translated artist with the title and the length", song: jihen,
			tracks: []Track{{ID: "a", Name: "私生活 - 新訳版", Artists: []string{"Tokyo Incidents"}, DurationMS: 218000},
				{ID: "b", Name: "私生活", Artists: []string{"Tokyo Incidents"}, DurationMS: 229546}}, want: "b"},
		{name: "a translated artist with the title but another length", song: jihen,
			tracks: []Track{{ID: "a", Name: "私生活", Artists: []string{"Tokyo Incidents"}, DurationMS: 240000}}},
		{name: "the artist over a cover of the same length", song: jihen,
			tracks: []Track{{ID: "a", Name: "私生活", Artists: []string{"Cover Band"}, DurationMS: 229546},
				{ID: "b", Name: "私生活", Artists: []string{"Tokyo Jihen"}, DurationMS: 231000}}, want: "b"},
		{name: "a short romaji title must match whole", song: Song{Title: "Kaikai Kitan", Artist: "Eve"},
			tracks: []Track{{ID: "a", Name: "愛", Artists: []string{"Eve"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := newTestPlayer(newFakeRepo(), newFakeAPI())
			got, ok, err := p.bestMatch(t.Context(), tt.tracks, tt.song)
			if err != nil {
				t.Fatalf("bestMatch: %v", err)
			}
			if got.ID != tt.want || ok != (tt.want != "") {
				t.Errorf("got %q, %v; want %q", got.ID, ok, tt.want)
			}
		})
	}
}

func TestBestMatchFailsWhenRomajiFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("morph: analysis failed")
	p := NewPlayer(newFakeRepo(), newFakeAPI(), fakeRomaji{err: failure}, func() time.Time { return clock })
	_, _, err := p.bestMatch(t.Context(), []Track{{ID: "a", Name: "私生活"}}, Song{Title: "Shiseikatsu"})
	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want the romanizer's error", err)
	}
}

func TestFolded(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{in: "Tōkyō Jihen", want: "tokyojihen"},
		{in: "toukyou jihen", want: "tokyojihen"},
		{in: "Tokyo Jihen", want: "tokyojihen"},
		{in: "Ookii", want: "oki"},
		{in: "Shiseikatsu", want: "shiseikatsu"},
	}
	for _, tt := range tests {
		if got := folded(tt.in); got != tt.want {
			t.Errorf("folded(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNowPlayingKeepsTheLengthOfATrackMatchedWithoutIt(t *testing.T) {
	t.Parallel()

	repo, api := newFakeRepo(), newFakeAPI()
	repo.tracks[1] = Track{ID: kazeTrack}
	api.state = &State{Playing: true, Track: Track{ID: kazeTrack, DurationMS: 179500}}
	if _, err := newTestPlayer(repo, api).NowPlaying(t.Context(), 1); err != nil {
		t.Fatalf("NowPlaying: %v", err)
	}
	if got := repo.tracks[1]; got.DurationMS != 179500 {
		t.Errorf("saved %+v, want the track's length kept", got)
	}
}

func TestSetOffset(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	p := newTestPlayer(repo, newFakeAPI())
	if err := p.SetOffset(t.Context(), 1, -5500); err != nil {
		t.Fatalf("SetOffset: %v", err)
	}
	if repo.offsets[1] != -5500 {
		t.Errorf("offset = %d, want -5500", repo.offsets[1])
	}
	if err := p.ResetOffset(t.Context(), 1); err != nil {
		t.Fatalf("ResetOffset: %v", err)
	}
	if _, ok := repo.offsets[1]; ok {
		t.Error("ResetOffset kept the offset")
	}
}

func TestSetOffsetRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		songID    int64
		offsetMS  int64
		wantField string
	}{
		{name: "no song", songID: 0, wantField: "songId"},
		{name: "more than a minute early", songID: 1, offsetMS: -60001, wantField: "offsetMs"},
		{name: "more than a minute late", songID: 1, offsetMS: 60001, wantField: "offsetMs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := newTestPlayer(newFakeRepo(), newFakeAPI()).SetOffset(t.Context(), tt.songID, tt.offsetMS)
			if got := invalidField(err); got != tt.wantField {
				t.Errorf("SetOffset error = %v, want invalid %s", err, tt.wantField)
			}
		})
	}
	err := newTestPlayer(newFakeRepo(), newFakeAPI()).SetOffset(t.Context(), 99, 0)
	if !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("SetOffset on a song not in your library = %v, want not found", err)
	}
}

func TestOtherLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		lyricsMS, trackMS int64
		want              int64
	}{
		{name: "the same length", lyricsMS: 191000, trackMS: 190500},
		{name: "a few seconds off", lyricsMS: 191000, trackMS: 185573, want: 191000},
		{name: "another cut of the song", lyricsMS: 249000, trackMS: 225920, want: 249000},
		{name: "no length for the lyrics", trackMS: 185500},
		{name: "no length for the track", lyricsMS: 185500},
	}
	for _, tt := range tests {
		if got := otherLength(tt.lyricsMS, tt.trackMS); got != tt.want {
			t.Errorf("%s: otherLength = %d, want %d", tt.name, got, tt.want)
		}
	}
}
