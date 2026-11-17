package songs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

type fakeSource struct {
	code, name string
	matches    []Match
	fetched    map[string]Fetched
	searchErr  error
	queries    []string
	fetches    []string
}

func (f *fakeSource) Code() string { return f.code }
func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) Search(_ context.Context, query string) ([]Match, error) {
	f.queries = append(f.queries, query)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	out := make([]Match, len(f.matches))
	copy(out, f.matches)
	return out, nil
}

func (f *fakeSource) Lyrics(_ context.Context, ref string) (Fetched, error) {
	f.fetches = append(f.fetches, ref)
	got, ok := f.fetched[ref]
	if !ok {
		return Fetched{}, fault.ErrNotFound
	}
	return got, nil
}

func newLRCLIBFake() *fakeSource {
	return &fakeSource{
		code: "lrclib", name: "LRCLIB",
		matches: []Match{
			{Source: "lrclib", SourceName: "LRCLIB", SourceRef: "11", Title: "星の歌", Artist: "Demo Artist", Synced: true},
			{Source: "lrclib", SourceName: "LRCLIB", SourceRef: "12", Title: "星の歌 (Live)", Artist: "Demo Artist"},
		},
		fetched: map[string]Fetched{
			"11": {
				Match:  Match{SourceRef: "11", Title: " 星の歌 ", Artist: "Demo Artist", DurationMS: 215000},
				Plain:  "夜の空に\n\n星が光る",
				Synced: "[00:12.00]夜の空に\n[00:15.50]星が光る",
			},
			"12": {Match: Match{SourceRef: "12", Title: "星の歌 (Live)"}, Plain: "夜の空に"},
			"13": {Match: Match{SourceRef: "13", Title: "Instrumental"}},
		},
	}
}

func TestSearchSongsValidatesTheQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
	}{
		{name: "empty", query: ""},
		{name: "only spaces", query: " 　 "},
		{name: "too long", query: strings.Repeat("歌", 201)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := newLRCLIBFake()
			_, err := SearchSongs(t.Context(), newFakeRepo(), []LyricsSource{src}, tt.query)

			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != "query" {
				t.Fatalf("err = %v, want invalid query", err)
			}
			if len(src.queries) != 0 {
				t.Errorf("searched %v, want no request for a bad query", src.queries)
			}
		})
	}
}

func TestSearchSongsMarksSongsAlreadyInTheLibrary(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	saved, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: "lrclib", SourceRef: "11"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	src := newLRCLIBFake()
	got, err := SearchSongs(t.Context(), repo, []LyricsSource{src}, "  ﾎｼ ")
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if want := []string{"ホシ"}; !reflect.DeepEqual(src.queries, want) {
		t.Errorf("queries = %q, want %q (normalized and trimmed)", src.queries, want)
	}
	if len(got.Matches) != 2 || got.Matches[0].SongID != saved.ID || got.Matches[1].SongID != 0 {
		t.Errorf("matches = %+v, want the first marked with song %d and the second not", got.Matches, saved.ID)
	}
	if len(got.Unavailable) != 0 {
		t.Errorf("unavailable = %v, want none", got.Unavailable)
	}
}

func TestSearchSongsKeepsGoingWhenASourceFails(t *testing.T) {
	t.Parallel()

	down := &fakeSource{code: "netease", name: "NetEase", searchErr: errors.New("songs: netease unreachable (boom)")}
	up := newLRCLIBFake()

	got, err := SearchSongs(t.Context(), newFakeRepo(), []LyricsSource{down, up}, "星")
	if err != nil {
		t.Fatalf("SearchSongs: %v", err)
	}
	if !reflect.DeepEqual(got.Unavailable, []string{"NetEase"}) {
		t.Errorf("unavailable = %v, want [NetEase]", got.Unavailable)
	}
	if len(got.Matches) != 2 {
		t.Errorf("matches = %d, want LRCLIB's 2", len(got.Matches))
	}
	if len(got.failures) != 1 {
		t.Errorf("failures kept for the log = %d, want 1", len(got.failures))
	}
}

func TestSearchSongsStopsWhenTheCallIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	src := &fakeSource{code: "lrclib", name: "LRCLIB", searchErr: fault.ErrCanceled}

	if _, err := SearchSongs(ctx, newFakeRepo(), []LyricsSource{src}, "星"); !errors.Is(err, fault.ErrCanceled) {
		t.Errorf("err = %v, want fault.ErrCanceled", err)
	}
}

func TestAddFromSourceSavesSyncedLyricsWithVerseBreaks(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	got, err := AddFromSource(t.Context(), repo, []LyricsSource{newLRCLIBFake()}, "lrclib", "11")
	if err != nil {
		t.Fatalf("AddFromSource: %v", err)
	}

	want := Song{
		ID: 1, Title: "星の歌", Artist: "Demo Artist", Source: "lrclib", SourceRef: "11", DurationMS: 215000,
		Lines: []Line{
			{Position: 0, Stanza: 0, Text: "夜の空に", StartMS: ms(12000)},
			{Position: 1, Stanza: 1, Text: "星が光る", StartMS: ms(15500)},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("song = %+v %s, want %+v %s", got, describe(got.Lines), want, describe(want.Lines))
	}
}

func TestAddFromSourceUsesPlainLyricsWhenNotSynced(t *testing.T) {
	t.Parallel()

	got, err := AddFromSource(t.Context(), newFakeRepo(), []LyricsSource{newLRCLIBFake()}, "lrclib", "12")
	if err != nil {
		t.Fatalf("AddFromSource: %v", err)
	}
	if len(got.Lines) != 1 || got.Lines[0].StartMS != nil {
		t.Errorf("lines = %s, want one line without a time", describe(got.Lines))
	}
}

func TestAddFromSourceReturnsTheSavedSongWithoutFetchingAgain(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	src := newLRCLIBFake()
	first, err := AddFromSource(t.Context(), repo, []LyricsSource{src}, "lrclib", "11")
	if err != nil {
		t.Fatalf("first AddFromSource: %v", err)
	}
	second, err := AddFromSource(t.Context(), repo, []LyricsSource{src}, "lrclib", "11")
	if err != nil {
		t.Fatalf("second AddFromSource: %v", err)
	}

	if second.ID != first.ID || len(repo.added) != 1 {
		t.Errorf("second add = song %d with %d saved, want song %d saved once", second.ID, len(repo.added), first.ID)
	}
	if len(src.fetches) != 1 {
		t.Errorf("fetched %d times, want once", len(src.fetches))
	}
}

func TestAddFromSourceRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		source    string
		ref       string
		wantField string
		wantErr   error
	}{
		{name: "unknown source", source: "spotify", ref: "1", wantField: "source"},
		{name: "empty ref", source: "lrclib", ref: " ", wantField: "ref"},
		{name: "a song with no lyrics", source: "lrclib", ref: "13", wantField: "lyrics"},
		{name: "a song the source doesn't have", source: "lrclib", ref: "99", wantErr: fault.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRepo()
			_, err := AddFromSource(t.Context(), repo, []LyricsSource{newLRCLIBFake()}, tt.source, tt.ref)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v, want %v", err, tt.wantErr)
				}
			} else {
				var invalid *fault.InvalidError
				if !errors.As(err, &invalid) || invalid.Field != tt.wantField {
					t.Errorf("err = %v, want invalid %s", err, tt.wantField)
				}
			}
			if len(repo.added) != 0 {
				t.Errorf("saved %d songs, want none", len(repo.added))
			}
		})
	}
}

func TestSearchIsOffWithoutSources(t *testing.T) {
	t.Parallel()

	_, err := SearchSongs(t.Context(), newFakeRepo(), nil, "星")
	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) || invalid.Field != "search" {
		t.Errorf("err = %v, want invalid search (turned off)", err)
	}
}

func TestAddFromSourceIsRefusedWhenItsSourceIsOff(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	_, err := AddFromSource(t.Context(), repo, nil, "lrclib", "11")
	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) || invalid.Field != "source" {
		t.Errorf("err = %v, want invalid source", err)
	}
}

func TestServiceListsTheSourcesThatAreOn(t *testing.T) {
	t.Parallel()

	if got := NewService(newFakeRepo()).SearchSources(); len(got) != 0 {
		t.Errorf("no sources = %v, want none", got)
	}
	got := NewService(newFakeRepo(), newLRCLIBFake()).SearchSources()
	if !reflect.DeepEqual(got, []string{"LRCLIB"}) {
		t.Errorf("sources = %v, want [LRCLIB]", got)
	}
}
