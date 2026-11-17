//go:build integration

package songs

import (
	"errors"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

func newSQLite(t *testing.T) *SQLite {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewSQLite(db.Querier())
}

func addSong(t *testing.T, repo *SQLite, title string) Song {
	t.Helper()
	song, err := repo.AddSong(t.Context(), Song{Title: title, Artist: "Demo Artist", Lyrics: "夜の空に\n星が光る",
		Source: SourceUser})
	if err != nil {
		t.Fatalf("AddSong(%s): %v", title, err)
	}
	return song
}

func TestSQLiteAddThenGet(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	added := addSong(t, repo, "星の歌")
	if added.ID <= 0 {
		t.Fatalf("ID = %d, want a positive ID", added.ID)
	}

	got, err := repo.SongByID(t.Context(), added.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if got != added {
		t.Errorf("got %+v, want %+v", got, added)
	}
}

func TestSQLiteSongsNewestFirst(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	first := addSong(t, repo, "星の歌")
	second := addSong(t, repo, "春の手紙")

	got, err := repo.Songs(t.Context())
	if err != nil {
		t.Fatalf("Songs: %v", err)
	}
	want := []Summary{
		{ID: second.ID, Title: "春の手紙", Artist: "Demo Artist", Source: SourceUser},
		{ID: first.ID, Title: "星の歌", Artist: "Demo Artist", Source: SourceUser},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSQLiteSongsEmptyIsNotNil(t *testing.T) {
	t.Parallel()

	got, err := newSQLite(t).Songs(t.Context())
	if err != nil {
		t.Fatalf("Songs: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %#v, want an empty, non-nil list", got)
	}
}

func TestSQLiteMissingSongIsNotFound(t *testing.T) {
	t.Parallel()

	_, err := newSQLite(t).SongByID(t.Context(), 99)
	if !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want fault.ErrNotFound", err)
	}
}

func TestSQLiteDelete(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	song := addSong(t, repo, "星の歌")

	if err := repo.DeleteSong(t.Context(), song.ID); err != nil {
		t.Fatalf("DeleteSong: %v", err)
	}
	if _, err := repo.SongByID(t.Context(), song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("after delete, err = %v, want fault.ErrNotFound", err)
	}
	if err := repo.DeleteSong(t.Context(), song.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("second delete, err = %v, want fault.ErrNotFound", err)
	}
}

func TestSQLiteNeverReusesADeletedID(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	first := addSong(t, repo, "星の歌")
	if err := repo.DeleteSong(t.Context(), first.ID); err != nil {
		t.Fatalf("DeleteSong: %v", err)
	}
	second := addSong(t, repo, "春の手紙")
	if second.ID == first.ID {
		t.Errorf("new song got the deleted song's ID %d", first.ID)
	}
}
