//go:build integration

package songs

import (
	"errors"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

func TestSQLitePurgesASongFromEveryLibrary(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	song, err := repo.AddSong(hana, Song{Title: "歌", Source: "lrclib", SourceRef: "7", Lines: sampleLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if err := repo.PurgeSong(kaze, song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze purging a song not in their library: %v, want not found", err)
	}
	if _, ok, err := repo.AddSavedSong(kaze, "lrclib", "7"); err != nil || !ok {
		t.Fatalf("AddSavedSong = %v, %v", ok, err)
	}
	if err := repo.SetWordTimes(kaze, song.ID, TimesByUser, []LineTimes{{Line: 0, Times: []WordTime{{0, 1000}}}}); err != nil {
		t.Fatalf("SetWordTimes: %v", err)
	}
	if err := repo.PurgeSong(hana, song.ID); err != nil {
		t.Fatalf("PurgeSong: %v", err)
	}
	if _, err := repo.SongByID(hana, song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("hana's song after purging: %v, want not found", err)
	}
	if _, err := repo.SongByID(kaze, song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze's copy after purging: %v, want not found", err)
	}
	if ids, err := repo.SongIDsBySource(kaze, "lrclib", []string{"7"}); err != nil || len(ids) != 0 {
		t.Errorf("saved from lrclib after purging = %v, %v; want none, so it's added fresh", ids, err)
	}
}
