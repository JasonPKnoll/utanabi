//go:build integration

package songs

import (
	"errors"
	"reflect"
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
	return NewSQLite(db)
}

func sampleLines() []Line {
	return []Line{
		{Position: 0, Stanza: 0, Text: "夜の空に"},
		{Position: 1, Stanza: 0, Text: "星が光る"},
		{Position: 2, Stanza: 1, Text: "君の声を探している"},
	}
}

func addSong(t *testing.T, repo *SQLite, title string) Song {
	t.Helper()
	song, err := repo.AddSong(t.Context(), Song{Title: title, Artist: "Demo Artist", Source: SourceUser, Lines: sampleLines()})
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
	if !reflect.DeepEqual(got, added) {
		t.Errorf("got %+v, want %+v", got, added)
	}
}

func TestSQLiteAddIsAllOrNothing(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	lines := []Line{{Position: 0, Stanza: 0, Text: "星"}, {Position: 0, Stanza: 0, Text: "same position"}}
	if _, err := repo.AddSong(t.Context(), Song{Title: "broken", Source: SourceUser, Lines: lines}); err == nil {
		t.Fatal("AddSong saved two lines in the same position, want an error")
	}

	list, err := repo.Songs(t.Context())
	if err != nil {
		t.Fatalf("Songs: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a failed add left %d songs behind, want none", len(list))
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
	if !reflect.DeepEqual(got, want) {
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

	var lines int
	err := repo.db.Querier().QueryRowContext(t.Context(), `SELECT count(*) FROM lyric_lines WHERE song_id = ?`, song.ID).
		Scan(&lines)
	if err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if lines != 0 {
		t.Errorf("%d lines left after deleting the song, want none", lines)
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
