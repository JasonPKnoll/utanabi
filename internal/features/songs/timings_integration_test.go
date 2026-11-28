//go:build integration

package songs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

func timesOf(t *testing.T, song Song) map[int][]WordTime {
	t.Helper()
	out := map[int][]WordTime{}
	for _, l := range song.Lines {
		if len(l.Times) > 0 {
			out[l.Position] = l.Times
		}
	}
	return out
}

func TestSQLiteSavesAndReadsWordTimes(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	song, err := repo.AddSong(t.Context(), Song{Title: "歌", Source: SourceUser, Lines: sampleLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	found := []LineTimes{
		{Line: 0, Times: []WordTime{{0, 1000}, {2, 1400}}},
		{Line: 1, Times: []WordTime{{0, 3000}}},
		{Line: 9, Times: []WordTime{{0, 9000}}},
	}
	if err := repo.SetWordTimes(t.Context(), song.ID, "amll", found); err != nil {
		t.Fatalf("SetWordTimes found: %v", err)
	}
	got, err := repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	want := map[int][]WordTime{0: {{0, 1000}, {2, 1400}}, 1: {{0, 3000}}}
	if !reflect.DeepEqual(timesOf(t, got), want) {
		t.Errorf("times = %+v, want %+v (a line the song hasn't is skipped)", timesOf(t, got), want)
	}
	if got.Lines[0].TimesBy != "amll" || got.Lines[2].Times != nil || got.Lines[2].TimesBy != "" {
		t.Errorf("lines = %+v, want the first by amll and an untimed one with none", got.Lines)
	}

	// Yours replace the found ones, and found ones never replace yours.
	mine := []LineTimes{{Line: 0, Times: []WordTime{{0, 1100}, {1, 1250}, {3, 1600}}}}
	if err := repo.SetWordTimes(t.Context(), song.ID, TimesByUser, mine); err != nil {
		t.Fatalf("SetWordTimes yours: %v", err)
	}
	if err := repo.SetWordTimes(t.Context(), song.ID, "amll", found[:2]); err != nil {
		t.Fatalf("SetWordTimes found again: %v", err)
	}
	got, err = repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(got.Lines[0].Times, mine[0].Times) || got.Lines[0].TimesBy != TimesByUser {
		t.Errorf("line 0 = %+v by %q, want yours kept", got.Lines[0].Times, got.Lines[0].TimesBy)
	}

	// No times removes a line's.
	if err := repo.SetWordTimes(t.Context(), song.ID, TimesByUser, []LineTimes{{Line: 0}}); err != nil {
		t.Fatalf("SetWordTimes none: %v", err)
	}
	if err := repo.ClearWordTimes(t.Context(), song.ID); err != nil {
		t.Fatalf("ClearWordTimes: %v", err)
	}
	got, err = repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if left := timesOf(t, got); len(left) != 0 {
		t.Errorf("after clearing = %+v, want none", left)
	}

	if err := repo.SetWordTimes(t.Context(), 999, TimesByUser, mine); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("no such song: %v, want not found", err)
	}
	if err := repo.ClearWordTimes(t.Context(), 999); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("clearing no such song: %v, want not found", err)
	}
}

func TestSQLiteKeepsEachUsersWordTimesTheirOwn(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	song, err := repo.AddSong(hana, Song{Title: "歌", Source: "lrclib", SourceRef: "7", Lines: sampleLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	mine := []LineTimes{{Line: 0, Times: []WordTime{{0, 1000}}}}
	if err := repo.SetWordTimes(kaze, song.ID, TimesByUser, mine); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze timing a song not in their library: %v, want not found", err)
	}
	if err := repo.ClearWordTimes(kaze, song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze clearing a song not in their library: %v, want not found", err)
	}
	if _, ok, err := repo.AddSavedSong(kaze, "lrclib", "7"); err != nil || !ok {
		t.Fatalf("AddSavedSong = %v, %v", ok, err)
	}
	if err := repo.SetWordTimes(hana, song.ID, TimesByUser, mine); err != nil {
		t.Fatalf("SetWordTimes: %v", err)
	}
	got, err := repo.SongByID(kaze, song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if left := timesOf(t, got); len(left) != 0 {
		t.Errorf("kaze sees %+v, want none of hana's", left)
	}
	if err := repo.ClearWordTimes(kaze, song.ID); err != nil {
		t.Fatalf("ClearWordTimes: %v", err)
	}
	got, err = repo.SongByID(hana, song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(timesOf(t, got), map[int][]WordTime{0: {{0, 1000}}}) {
		t.Errorf("hana's after kaze cleared theirs = %+v, want hana's kept", timesOf(t, got))
	}
}
