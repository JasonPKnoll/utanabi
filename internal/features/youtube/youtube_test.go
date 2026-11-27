package youtube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
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
}

func (f *fakeSearch) Videos(_ context.Context, query string) ([]Video, error) {
	f.searches++
	f.queries = append(f.queries, query)
	return f.found, f.err
}

func (f *fakeSearch) Video(_ context.Context, id string) (Video, error) {
	if f.err != nil {
		return Video{}, f.err
	}
	for _, v := range f.found {
		if v.ID == id {
			return v, nil
		}
	}
	return Video{}, fault.ErrNotFound
}

func newFinder(repo *fakeRepo, search Search) *Finder {
	return NewFinder(repo, search, func() time.Time { return clock })
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
