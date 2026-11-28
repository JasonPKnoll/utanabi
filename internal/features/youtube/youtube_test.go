package youtube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

var clock = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

type fakeRepo struct {
	songs  map[int64]Song
	videos map[int64]Video
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		songs:  map[int64]Song{1: {Title: "Matsuri", Artist: "Fujii Kaze", DurationMS: 249000}},
		videos: map[int64]Video{},
	}
}

func (f *fakeRepo) Song(_ context.Context, id int64) (Song, error) {
	s, ok := f.songs[id]
	if !ok {
		return Song{}, fault.ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) VideoFor(_ context.Context, id int64) (Video, error) {
	v, ok := f.videos[id]
	if !ok {
		return Video{}, fault.ErrNotFound
	}
	return v, nil
}

func (f *fakeRepo) SaveVideo(_ context.Context, id int64, v Video, _ time.Time) error {
	f.videos[id] = v
	return nil
}

func (f *fakeRepo) SaveOffset(_ context.Context, id, offsetMS int64) error {
	v, ok := f.videos[id]
	if !ok {
		return fault.ErrNotFound
	}
	v.OffsetMS = offsetMS
	f.videos[id] = v
	return nil
}

func (f *fakeRepo) DeleteVideo(_ context.Context, id int64) error {
	delete(f.videos, id)
	return nil
}

type fakeSearch struct {
	found    []Video
	err      error
	searches int
	queries  []string
	keys     []string
}

func (f *fakeSearch) Videos(_ context.Context, key redact.Secret, query string) ([]Video, error) {
	f.searches++
	f.queries = append(f.queries, query)
	f.keys = append(f.keys, key.Reveal())
	return f.found, f.err
}

func (f *fakeSearch) Video(_ context.Context, key redact.Secret, id string) (Video, error) {
	f.keys = append(f.keys, key.Reveal())
	if f.err != nil {
		return Video{}, f.err
	}
	if id == checkVideo {
		return Video{ID: checkVideo, Title: "Me at the zoo"}, nil
	}
	for _, v := range f.found {
		if v.ID == id {
			return v, nil
		}
	}
	return Video{}, fault.ErrNotFound
}

// fakeKeys keeps the admin's key in memory.
type fakeKeys struct{ key string }

func (f *fakeKeys) APIKey(context.Context) (redact.Secret, error) {
	if f.key == "" {
		return redact.Secret{}, fault.ErrNotFound
	}
	return redact.New(f.key), nil
}

func (f *fakeKeys) SaveAPIKey(_ context.Context, key redact.Secret, _ time.Time) error {
	f.key = key.Reveal()
	return nil
}

func (f *fakeKeys) DeleteAPIKey(context.Context) error {
	f.key = ""
	return nil
}

const (
	adminKey  = "AIzaAdminKeyAdminKeyAdminKeyAdmin12"
	serverKey = "AIzaServerKeyServerKeyServerKeyServ"
)

// newFinder is a finder that searches with the admin's key when search is given, and can't search otherwise.
func newFinder(repo *fakeRepo, search Search) *Finder {
	keys := &fakeKeys{}
	if search != nil {
		keys.key = adminKey
	}
	return NewFinder(repo, keys, redact.Secret{}, search, func() time.Time { return clock })
}

func admin(ctx context.Context) context.Context {
	return identity.Into(ctx, identity.User{ID: 1, Name: "hana", Admin: true})
}

func invalidField(err error) string {
	var invalid *fault.InvalidError
	if errors.As(err, &invalid) {
		return invalid.Field
	}
	return ""
}

var (
	studio   = Video{ID: "AAAAAAAAAAA", Title: "Matsuri", Channel: "Fujii Kaze - Topic", DurationMS: 225000}
	official = Video{ID: "BBBBBBBBBBB", Title: "Fujii Kaze - Matsuri (Official Video)", Channel: "Fujii Kaze",
		DurationMS: 248500}
	live = Video{ID: "CCCCCCCCCCC", Title: "Matsuri (Live)", Channel: "Fujii Kaze", DurationMS: 320000}
)

func TestParseLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		link string
		want string
	}{
		{link: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{link: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s&list=PL1", want: "dQw4w9WgXcQ"},
		{link: "https://m.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{link: "https://music.youtube.com/watch?v=dQw4w9WgXcQ&si=abc", want: "dQw4w9WgXcQ"},
		{link: "https://youtu.be/dQw4w9WgXcQ?si=abc", want: "dQw4w9WgXcQ"},
		{link: "youtu.be/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{link: "https://www.youtube.com/shorts/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{link: "https://www.youtube.com/embed/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{link: "https://www.youtube.com/live/dQw4w9WgXcQ?feature=share", want: "dQw4w9WgXcQ"},
		{link: "  dQw4w9WgXcQ  ", want: "dQw4w9WgXcQ"},
		{link: ""},
		{link: "https://example.com/watch?v=dQw4w9WgXcQ"},
		{link: "https://www.youtube.com/watch?v=short"},
		{link: "https://www.youtube.com/@fujiikaze"},
		{link: "javascript:alert(1)"},
		{link: "https://youtu.be/dQw4w9WgXcQ/../../etc"},
	}
	for _, tt := range tests {
		t.Run(tt.link, func(t *testing.T) {
			t.Parallel()
			got, err := ParseLink(tt.link)
			if tt.want == "" {
				if invalidField(err) != "link" {
					t.Errorf("ParseLink = %q, %v; want an invalid link", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("ParseLink = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestBestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		videos   []Video
		lyricsMS int64
		want     string
	}{
		{name: "the one the lyrics' length", videos: []Video{studio, official, live}, lyricsMS: 249000,
			want: official.ID},
		{name: "the most relevant when none is that long", videos: []Video{live, studio}, lyricsMS: 200000,
			want: live.ID},
		{name: "the most relevant when the lyrics have no length", videos: []Video{studio, official}, want: studio.ID},
		{name: "none found", videos: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := bestMatch(tt.videos, tt.lyricsMS)
			if got.ID != tt.want || ok != (tt.want != "") {
				t.Errorf("bestMatch = %q, %v; want %q", got.ID, ok, tt.want)
			}
		})
	}
}

func TestVideoSearchesOnceAndKeepsIt(t *testing.T) {
	t.Parallel()

	repo, search := newFakeRepo(), &fakeSearch{found: []Video{studio, official}}
	f := newFinder(repo, search)
	for range 3 {
		got, err := f.Video(t.Context(), 1)
		if err != nil {
			t.Fatalf("Video: %v", err)
		}
		if got.ID != official.ID {
			t.Errorf("Video = %+v, want the official video, the lyrics' length", got)
		}
	}
	if search.searches != 1 || search.queries[0] != "Fujii Kaze Matsuri" {
		t.Errorf("%d searches for %q, want one for the artist and title", search.searches, search.queries)
	}
}

func TestVideoWithoutSearchIsEmpty(t *testing.T) {
	t.Parallel()

	got, err := newFinder(newFakeRepo(), nil).Video(t.Context(), 1)
	if err != nil || got.ID != "" {
		t.Errorf("Video = %+v, %v; want none, to paste a link", got, err)
	}
}

func TestVideoWhenSearchFails(t *testing.T) {
	t.Parallel()

	_, err := newFinder(newFakeRepo(), &fakeSearch{err: ErrQuota}).Video(t.Context(), 1)
	if invalidField(err) != "youtube" {
		t.Errorf("with the quota spent: %v, want to paste a link instead", err)
	}
	failure := errors.New("youtube: can't reach YouTube")
	if _, err := newFinder(newFakeRepo(), &fakeSearch{err: failure}).Video(t.Context(), 1); !errors.Is(err, failure) {
		t.Errorf("err = %v, want the search's", err)
	}
}

func TestVideoForASongNotInTheLibrary(t *testing.T) {
	t.Parallel()

	if _, err := newFinder(newFakeRepo(), &fakeSearch{}).Video(t.Context(), 9); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want not found", err)
	}
	if _, err := newFinder(newFakeRepo(), &fakeSearch{}).Video(t.Context(), 0); invalidField(err) != "songId" {
		t.Errorf("err = %v, want invalid songId", err)
	}
}

func TestSetVideo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		search Search
		want   Video
	}{
		{name: "looked up when search can", search: &fakeSearch{found: []Video{official}},
			want: Video{ID: official.ID, Title: official.Title, Channel: official.Channel,
				DurationMS: official.DurationMS, Chosen: true}},
		{name: "just its ID without search", search: nil, want: Video{ID: official.ID, Chosen: true}},
		{name: "just its ID when search has run out", search: &fakeSearch{err: ErrQuota},
			want: Video{ID: official.ID, Chosen: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeRepo()
			repo.videos[1] = Video{ID: studio.ID, OffsetMS: -900}
			got, err := newFinder(repo, tt.search).SetVideo(t.Context(), 1, "https://youtu.be/"+official.ID)
			if err != nil {
				t.Fatalf("SetVideo: %v", err)
			}
			if got != tt.want || repo.videos[1] != tt.want {
				t.Errorf("got %+v, saved %+v; want %+v, with the old offset gone", got, repo.videos[1], tt.want)
			}
		})
	}
}

func TestSetVideoRejectsBadInput(t *testing.T) {
	t.Parallel()

	f := newFinder(newFakeRepo(), nil)
	if _, err := f.SetVideo(t.Context(), 1, "https://example.com"); invalidField(err) != "link" {
		t.Errorf("a link that isn't YouTube's: %v, want invalid link", err)
	}
	if _, err := f.SetVideo(t.Context(), 9, official.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("a song not in the library: %v, want not found", err)
	}
}

func TestSetOffsetAndForget(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	f := newFinder(repo, nil)
	if err := f.SetOffset(t.Context(), 1, -900); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("an offset with no video: %v, want not found", err)
	}
	repo.videos[1] = official
	if err := f.SetOffset(t.Context(), 1, -900); err != nil || repo.videos[1].OffsetMS != -900 {
		t.Errorf("SetOffset: %v, offset %d; want -900", err, repo.videos[1].OffsetMS)
	}
	if err := f.SetOffset(t.Context(), 1, 60001); invalidField(err) != "offsetMs" {
		t.Errorf("more than a minute: %v, want invalid offsetMs", err)
	}
	if err := f.ForgetVideo(t.Context(), 1); err != nil {
		t.Fatalf("ForgetVideo: %v", err)
	}
	if _, ok := repo.videos[1]; ok {
		t.Error("ForgetVideo kept the video")
	}
}

func TestTheKeySearchesUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admin     string
		server    string
		wantKey   string
		wantFrom  string
		searchble bool
	}{
		{name: "the admin's, over the server's", admin: adminKey, server: serverKey, wantKey: adminKey,
			wantFrom: KeyFromSettings, searchble: true},
		{name: "the server's when no admin set one", server: serverKey, wantKey: serverKey, wantFrom: KeyFromServer,
			searchble: true},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			search := &fakeSearch{found: []Video{official}}
			f := NewFinder(newFakeRepo(), &fakeKeys{key: tt.admin}, redact.New(tt.server), search,
				func() time.Time { return clock })
			status, err := f.Status(t.Context())
			if err != nil || status.KeyFrom != tt.wantFrom || status.Searchable != tt.searchble || status.CanSetKey {
				t.Errorf("Status = %+v, %v; want from %q, and no key setting for someone not an admin", status, err,
					tt.wantFrom)
			}
			if _, err := f.Video(t.Context(), 1); err != nil {
				t.Fatalf("Video: %v", err)
			}
			if tt.wantKey == "" && search.searches != 0 {
				t.Errorf("searched with no key")
			}
			if tt.wantKey != "" && (len(search.keys) != 1 || search.keys[0] != tt.wantKey) {
				t.Errorf("searched with %q, want %q", search.keys, tt.wantKey)
			}
		})
	}
}

func TestSetAPIKey(t *testing.T) {
	t.Parallel()

	keys, search := &fakeKeys{}, &fakeSearch{}
	f := NewFinder(newFakeRepo(), keys, redact.Secret{}, search, func() time.Time { return clock })
	if err := f.SetAPIKey(admin(t.Context()), "  "+adminKey+"\n"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	if keys.key != adminKey || search.keys[0] != adminKey {
		t.Errorf("kept %q after checking %q, want the key, trimmed, checked with Google first", keys.key, search.keys)
	}
	status, err := f.Status(admin(t.Context()))
	if err != nil || !status.CanSetKey || status.KeyFrom != KeyFromSettings {
		t.Errorf("Status for an admin = %+v, %v", status, err)
	}
	if err := f.ClearAPIKey(admin(t.Context())); err != nil || keys.key != "" {
		t.Errorf("ClearAPIKey: %v, key %q; want it gone", err, keys.key)
	}
}

func TestSetAPIKeyRefuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       func(context.Context) context.Context
		key       string
		googleErr error
		want      func(error) bool
	}{
		{name: "someone not signed in", ctx: func(c context.Context) context.Context { return c }, key: adminKey,
			want: func(err error) bool { return errors.Is(err, fault.ErrUnauthorized) }},
		{name: "someone not an admin",
			ctx: func(c context.Context) context.Context { return identity.Into(c, identity.User{ID: 2, Name: "kaze"}) },
			key: adminKey, want: func(err error) bool { return errors.Is(err, fault.ErrForbidden) }},
		{name: "text that isn't a key", ctx: admin, key: "not a key",
			want: func(err error) bool { return invalidField(err) == "key" }},
		{name: "a key Google turns down", ctx: admin, key: adminKey, googleErr: ErrKeyRejected,
			want: func(err error) bool { return invalidField(err) == "key" }},
		{name: "a key whose project hasn't switched on YouTube", ctx: admin, key: adminKey, googleErr: ErrAPIOff,
			want: func(err error) bool { return invalidField(err) == "key" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			keys := &fakeKeys{}
			f := NewFinder(newFakeRepo(), keys, redact.Secret{}, &fakeSearch{err: tt.googleErr},
				func() time.Time { return clock })
			if err := f.SetAPIKey(tt.ctx(t.Context()), tt.key); !tt.want(err) {
				t.Errorf("err = %v", err)
			}
			if keys.key != "" {
				t.Error("kept a key it refused")
			}
		})
	}
	if err := NewFinder(newFakeRepo(), &fakeKeys{key: adminKey}, redact.Secret{}, &fakeSearch{},
		func() time.Time { return clock }).ClearAPIKey(t.Context()); !errors.Is(err, fault.ErrUnauthorized) {
		t.Errorf("ClearAPIKey with no one signed in: %v", err)
	}
}

func TestSetAPIKeyKeepsAKeyOutOfQuota(t *testing.T) {
	t.Parallel()

	keys := &fakeKeys{}
	f := NewFinder(newFakeRepo(), keys, redact.Secret{}, &fakeSearch{err: ErrQuota}, func() time.Time { return clock })
	if err := f.SetAPIKey(admin(t.Context()), adminKey); err != nil || keys.key != adminKey {
		t.Errorf("SetAPIKey: %v, kept %q; want a key out of quota today kept", err, keys.key)
	}
}

func TestVideoWithABrokenKey(t *testing.T) {
	t.Parallel()

	_, err := newFinder(newFakeRepo(), &fakeSearch{err: ErrKeyRejected}).Video(t.Context(), 1)
	if invalidField(err) != "youtube" {
		t.Errorf("err = %v, want one saying an admin can replace the key", err)
	}
}
