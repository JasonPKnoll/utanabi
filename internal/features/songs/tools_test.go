package songs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

func asAdmin(ctx context.Context) context.Context {
	return identity.Into(ctx, identity.User{ID: 1, Name: "hana", Admin: true})
}

func TestToolsAreForAdminsOnly(t *testing.T) {
	t.Parallel()

	kit := Toolkit{Repo: newFakeRepo(), Analyzer: fakeAnalyzer{}, Dict: noMeanings(t)}
	for name, ctx := range map[string]context.Context{
		"someone else":     identity.Into(t.Context(), identity.User{ID: 2, Name: "kaze"}),
		"no one signed in": t.Context(),
	} {
		if _, err := kit.TimedSongs(ctx, ""); err == nil {
			t.Errorf("%s: TimedSongs allowed", name)
		}
		if _, err := kit.AddTimedSong(ctx, "a.ttml"); err == nil {
			t.Errorf("%s: AddTimedSong allowed", name)
		}
		if _, err := kit.ReaddSong(ctx, 1); err == nil {
			t.Errorf("%s: ReaddSong allowed", name)
		}
	}
	if _, err := kit.TimedSongs(identity.Into(t.Context(), identity.User{ID: 2}), ""); !errors.Is(err, fault.ErrForbidden) {
		t.Errorf("err = %v, want forbidden", err)
	}
}

func TestTimedSongsFindsByTitleOrArtistNewestFirst(t *testing.T) {
	t.Parallel()

	src := &fakeTimings{refs: []SheetRef{
		{ID: "1.ttml", Title: "夜に駆ける", Artist: "YOASOBI"},
		{ID: "2.ttml", Title: "うっせぇわ", Artist: "Ado"},
		{ID: "3.ttml", Title: "アイドル", Artist: "YOASOBI"},
	}}
	kit := Toolkit{Timings: []TimingSource{src}}
	got, err := kit.TimedSongs(asAdmin(t.Context()), " yoasobi ")
	if err != nil {
		t.Fatalf("TimedSongs: %v", err)
	}
	want := []TimedSong{{Sheet: "3.ttml", Title: "アイドル", Artist: "YOASOBI"}, {Sheet: "1.ttml", Title: "夜に駆ける", Artist: "YOASOBI"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TimedSongs = %+v, want %+v", got, want)
	}
	if all, err := kit.TimedSongs(asAdmin(t.Context()), ""); err != nil || len(all) != 3 || all[0].Sheet != "3.ttml" {
		t.Errorf("no query = %+v, %v; want all three, newest first", all, err)
	}
	if _, err := kit.TimedSongs(asAdmin(t.Context()), strings.Repeat("あ", maxQueryLen+1)); err == nil {
		t.Error("an over-long query was accepted")
	}
}

// lrcOf writes a song's lines as synced lyrics.
func lrcOf(song Song) string {
	var b strings.Builder
	for _, l := range song.Lines {
		fmt.Fprintf(&b, "[00:%02d.00]%s\n", *l.StartMS/1000, l.Text)
	}
	return b.String()
}

func TestAddTimedSongPicksTheLyricsItsTimesFit(t *testing.T) {
	t.Parallel()

	song, sheet := timedSong(8)
	timings := &fakeTimings{refs: []SheetRef{{ID: "yoru.ttml", Title: "夜に駆ける", Artist: "YOASOBI"}},
		sheets: map[string]TimedSheet{"yoru.ttml": sheet}}
	lyrics := &fakeSource{
		code: "lrclib", name: "LRCLIB",
		matches: []Match{
			{Source: "lrclib", SourceRef: "21", Title: "夜に駆ける (plain)", Synced: false},
			{Source: "lrclib", SourceRef: "20", Title: "YOASOBI 夜に駆ける (inst)", Synced: true},
			{Source: "lrclib", SourceRef: "24", Title: "夜に駆ける - YOASOBI", Synced: true},
			{Source: "lrclib", SourceRef: "22", Title: "夜に駆ける (another version)", Synced: true},
			{Source: "lrclib", SourceRef: "23", Title: "夜に駆ける", Synced: true},
		},
		fetched: map[string]Fetched{
			"22": {Match: Match{SourceRef: "22", Title: "夜に駆ける"}, Synced: "[00:01.00]ちがう\n[00:02.00]うたです"},
			"23": {Match: Match{SourceRef: "23", Title: "夜に駆ける", Artist: "YOASOBI"}, Synced: lrcOf(song)},
			// The same lyrics, timed the same, under a longer title, and a backing track: neither is picked.
			"24": {Match: Match{SourceRef: "24", Title: "夜に駆ける - YOASOBI"}, Synced: lrcOf(song)},
			"20": {Match: Match{SourceRef: "20", Title: "YOASOBI 夜に駆ける (inst)"}, Synced: lrcOf(song)},
		},
	}
	repo := newFakeRepo()
	kit := Toolkit{Repo: repo, Analyzer: fakeAnalyzer{}, Dict: noMeanings(t), Sources: []LyricsSource{lyrics},
		Timings: []TimingSource{timings}}

	got, err := kit.AddTimedSong(asAdmin(t.Context()), "yoru.ttml")
	if err != nil {
		t.Fatalf("AddTimedSong: %v", err)
	}
	if got.Title != "夜に駆ける" || got.Lines != 8 || got.Timed != 8 || got.Source != "AMLL TTML DB" {
		t.Errorf("AddTimedSong = %+v, want 8 of 8 lines timed", got)
	}
	if added := repo.songs[got.SongID]; added.SourceRef != "23" {
		t.Errorf("added LRCLIB %q, want 23, the version the times fit with the song's own title", added.SourceRef)
	}
	if slices.Contains(lyrics.fetches, "20") {
		t.Error("read the backing track's lyrics, want it skipped")
	}
	if want := []string{"夜に駆ける YOASOBI"}; !reflect.DeepEqual(lyrics.queries, want) {
		t.Errorf("searched %v, want %v", lyrics.queries, want)
	}
}

func TestAddTimedSongRefusesLyricsItsTimesDontFit(t *testing.T) {
	t.Parallel()

	_, sheet := timedSong(8)
	timings := &fakeTimings{refs: []SheetRef{{ID: "yoru.ttml", Title: "夜に駆ける"}},
		sheets: map[string]TimedSheet{"yoru.ttml": sheet}}
	lyrics := &fakeSource{code: "lrclib", name: "LRCLIB",
		matches: []Match{{Source: "lrclib", SourceRef: "22", Synced: true}},
		fetched: map[string]Fetched{"22": {Synced: "[00:01.00]ちがう"}}}
	repo := newFakeRepo()
	kit := Toolkit{Repo: repo, Analyzer: fakeAnalyzer{}, Dict: noMeanings(t), Sources: []LyricsSource{lyrics},
		Timings: []TimingSource{timings}}
	var invalid *fault.InvalidError
	if _, err := kit.AddTimedSong(asAdmin(t.Context()), "yoru.ttml"); !errors.As(err, &invalid) {
		t.Errorf("err = %v, want invalid", err)
	}
	if len(repo.added) != 0 {
		t.Errorf("added %d songs, want none", len(repo.added))
	}
	if _, err := kit.AddTimedSong(asAdmin(t.Context()), "missing.ttml"); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("a sheet the database hasn't: %v, want not found", err)
	}
}

func TestReaddSongAddsItAgainFresh(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[5] = Song{ID: 5, Title: "星の歌", Source: "lrclib", SourceRef: "11",
		Lines: []Line{{Position: 0, Text: "古い行"}}}
	kit := Toolkit{Repo: repo, Analyzer: fakeAnalyzer{}, Dict: noMeanings(t), Sources: []LyricsSource{newLRCLIBFake()},
		Timings: []TimingSource{&fakeTimings{}}}

	got, err := kit.ReaddSong(asAdmin(t.Context()), 5)
	if err != nil {
		t.Fatalf("ReaddSong: %v", err)
	}
	if !reflect.DeepEqual(repo.purged, []int64{5}) {
		t.Errorf("purged %v, want the old song", repo.purged)
	}
	if got.SongID == 5 || got.Lines != 2 || got.Timed != 0 {
		t.Errorf("ReaddSong = %+v, want a new song with LRCLIB's two lines", got)
	}
}

func TestReaddSongRefusesYourOwnLyrics(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[5] = Song{ID: 5, Title: "歌", Source: SourceUser}
	kit := Toolkit{Repo: repo, Analyzer: fakeAnalyzer{}, Dict: noMeanings(t), Sources: []LyricsSource{newLRCLIBFake()}}
	var invalid *fault.InvalidError
	if _, err := kit.ReaddSong(asAdmin(t.Context()), 5); !errors.As(err, &invalid) {
		t.Errorf("err = %v, want invalid", err)
	}
	if len(repo.purged) != 0 {
		t.Errorf("purged %v, want nothing", repo.purged)
	}
	if _, err := kit.ReaddSong(asAdmin(t.Context()), 99); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("no such song: %v, want not found", err)
	}
}
