//go:build integration

package songs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// voice is a sung reading for 声 in 君の声を探している.
func voice(reading string) SungReading {
	return SungReading{Line: 0, FirstToken: 2, LastToken: 2, Surface: "声", Reading: reading}
}

func sungOf(t *testing.T, ctx context.Context, repo *SQLite, id int64) []SungReading {
	t.Helper()
	song, err := repo.SongByID(ctx, id)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	return song.Lines[0].Sung
}

func TestSQLiteSetsReplacesAndRemovesASungReading(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	song, err := repo.AddSong(t.Context(), Song{Title: "歌", Source: SourceUser, Lines: []Line{searchingLine()}})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	if err := repo.SetSungReading(t.Context(), song.ID, voice("うた")); err != nil {
		t.Fatalf("SetSungReading: %v", err)
	}
	if got := sungOf(t, t.Context(), repo, song.ID); !reflect.DeepEqual(got, []SungReading{voice("うた")}) {
		t.Errorf("after setting = %+v", got)
	}
	if err := repo.SetSungReading(t.Context(), song.ID, voice("ボイス")); err != nil {
		t.Fatalf("SetSungReading again: %v", err)
	}
	if got := sungOf(t, t.Context(), repo, song.ID); !reflect.DeepEqual(got, []SungReading{voice("ボイス")}) {
		t.Errorf("after replacing = %+v, want only the new one", got)
	}
	if err := repo.SetSungReading(t.Context(), song.ID, voice("")); err != nil {
		t.Fatalf("SetSungReading empty: %v", err)
	}
	if got := sungOf(t, t.Context(), repo, song.ID); len(got) != 0 {
		t.Errorf("after removing = %+v, want none", got)
	}

	if err := repo.SetSungReading(t.Context(), 999, voice("うた")); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("no such song: %v, want not found", err)
	}
	missing := voice("うた")
	missing.Line = 7
	if err := repo.SetSungReading(t.Context(), song.ID, missing); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("no such line: %v, want not found", err)
	}
}

func TestSQLiteKeepsEachUsersSungReadingsTheirOwn(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	song, err := repo.AddSong(hana, Song{Title: "歌", Source: "lrclib", SourceRef: "7", Lines: []Line{searchingLine()}})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if err := repo.SetSungReading(kaze, song.ID, voice("うた")); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze setting a reading on a song she hasn't added: %v, want not found", err)
	}
	if _, ok, err := repo.AddSavedSong(kaze, "lrclib", "7"); err != nil || !ok {
		t.Fatalf("AddSavedSong = %v, %v", ok, err)
	}

	if err := repo.SetSungReading(hana, song.ID, voice("うた")); err != nil {
		t.Fatalf("hana SetSungReading: %v", err)
	}
	if got := sungOf(t, kaze, repo, song.ID); len(got) != 0 {
		t.Errorf("kaze sees %+v, want none of hana's readings", got)
	}
	if err := repo.SetSungReading(kaze, song.ID, voice("ボイス")); err != nil {
		t.Fatalf("kaze SetSungReading: %v", err)
	}
	if got := sungOf(t, hana, repo, song.ID); !reflect.DeepEqual(got, []SungReading{voice("うた")}) {
		t.Errorf("hana's readings = %+v, want hers unchanged", got)
	}
	if got := sungOf(t, kaze, repo, song.ID); !reflect.DeepEqual(got, []SungReading{voice("ボイス")}) {
		t.Errorf("kaze's readings = %+v, want hers", got)
	}
}
