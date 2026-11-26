//go:build integration

package songs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

func newSQLite(t *testing.T) *SQLite {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewSQLite(db, identity.OnDevice())
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

func addOnlineSong(t *testing.T, repo *SQLite, ref string) Song {
	t.Helper()
	start := int64(12000)
	song, err := repo.AddSong(t.Context(), Song{
		Title: "星の歌", Artist: "Demo Artist", Source: "lrclib", SourceRef: ref, DurationMS: 215000,
		Lines: []Line{
			{Position: 0, Stanza: 0, Text: "夜の空に", StartMS: &start},
			{Position: 1, Stanza: 0, Text: "星が光る"},
		},
	})
	if err != nil {
		t.Fatalf("AddSong(lrclib %s): %v", ref, err)
	}
	return song
}

func TestSQLiteKeepsSourceDetailsAndLineTimes(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	added := addOnlineSong(t, repo, "11")

	got, err := repo.SongByID(t.Context(), added.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(got, added) {
		t.Errorf("got %+v %s, want %+v %s", got, describe(got.Lines), added, describe(added.Lines))
	}
}

func TestSQLiteSongIDsBySource(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	eleven := addOnlineSong(t, repo, "11")
	twelve := addOnlineSong(t, repo, "12")
	addSong(t, repo, "your own lyrics")

	got, err := repo.SongIDsBySource(t.Context(), "lrclib", []string{"11", "12", "99"})
	if err != nil {
		t.Fatalf("SongIDsBySource: %v", err)
	}
	want := map[string]int64{"11": eleven.ID, "12": twelve.ID}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	other, err := repo.SongIDsBySource(t.Context(), "netease", []string{"11"})
	if err != nil {
		t.Fatalf("SongIDsBySource(netease): %v", err)
	}
	if len(other) != 0 {
		t.Errorf("another source's IDs matched: %v", other)
	}

	none, err := repo.SongIDsBySource(t.Context(), "lrclib", nil)
	if err != nil || len(none) != 0 {
		t.Errorf("no refs = %v, %v, want an empty map", none, err)
	}
}

func TestSQLiteSameOnlineSongTwiceIsAConflict(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	addOnlineSong(t, repo, "11")
	_, err := repo.AddSong(t.Context(), Song{Title: "again", Source: "lrclib", SourceRef: "11", Lines: sampleLines()})
	if !errors.Is(err, fault.ErrConflict) {
		t.Errorf("err = %v, want fault.ErrConflict", err)
	}
}

// twoUsers is a repository in server mode, with two users and a context for each.
func twoUsers(t *testing.T) (*SQLite, context.Context, context.Context) {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, err := db.Querier().ExecContext(t.Context(), `INSERT INTO users (id, handle, name, created_at) VALUES
		(1, randomblob(32), 'hana', 1), (2, randomblob(32), 'kaze', 1)`)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}
	hana := identity.Into(t.Context(), identity.User{ID: 1, Name: "hana"})
	kaze := identity.Into(t.Context(), identity.User{ID: 2, Name: "kaze"})
	return NewSQLite(db, identity.Accounts()), hana, kaze
}

func TestSQLiteKeepsEachUsersLibraryTheirOwn(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	mine, err := repo.AddSong(hana, Song{Title: "花の歌", Source: SourceUser, Lines: sampleLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	if list, err := repo.Songs(kaze); err != nil || len(list) != 0 {
		t.Errorf("kaze's songs = %+v, %v, want none", list, err)
	}
	if list, err := repo.Songs(hana); err != nil || len(list) != 1 || list[0].ID != mine.ID {
		t.Errorf("hana's songs = %+v, %v, want hers", list, err)
	}
	if _, err := repo.SongByID(kaze, mine.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze reading hana's song: %v, want not found", err)
	}
	if err := repo.SaveAnalysis(kaze, mine.ID, nil); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze saving analysis of hana's song: %v, want not found", err)
	}
	if err := repo.DeleteSong(kaze, mine.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze deleting hana's song: %v, want not found", err)
	}
	if _, err := repo.SongByID(hana, mine.ID); err != nil {
		t.Errorf("hana's song after kaze tried to delete it: %v", err)
	}
}

func TestSQLiteSharesASongFromASourceBetweenLibraries(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	if _, ok, err := repo.AddSavedSong(kaze, "lrclib", "42"); err != nil || ok {
		t.Fatalf("AddSavedSong before anyone saved it = %v, %v, want not found", ok, err)
	}
	saved, err := repo.AddSong(hana, Song{Title: "夜に駆ける", Source: "lrclib", SourceRef: "42", Lines: sampleLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if found, err := repo.SongIDsBySource(kaze, "lrclib", []string{"42"}); err != nil || len(found) != 0 {
		t.Errorf("kaze's saved songs from lrclib before adding = %v, %v, want none", found, err)
	}

	id, ok, err := repo.AddSavedSong(kaze, "lrclib", "42")
	if err != nil || !ok || id != saved.ID {
		t.Fatalf("AddSavedSong = %d, %v, %v, want hana's song %d", id, ok, err, saved.ID)
	}
	if again, ok, err := repo.AddSavedSong(kaze, "lrclib", "42"); err != nil || !ok || again != saved.ID {
		t.Errorf("AddSavedSong a second time = %d, %v, %v, want the same song", again, ok, err)
	}
	if found, err := repo.SongIDsBySource(kaze, "lrclib", []string{"42"}); err != nil || found["42"] != saved.ID {
		t.Errorf("kaze's saved songs from lrclib = %v, %v, want song %d", found, err, saved.ID)
	}

	// Each lets go in turn: the song stays while either has it.
	if err := repo.DeleteSong(hana, saved.ID); err != nil {
		t.Fatalf("hana deleting: %v", err)
	}
	if _, err := repo.SongByID(kaze, saved.ID); err != nil {
		t.Errorf("kaze's song after hana deleted hers: %v", err)
	}
	if err := repo.DeleteSong(kaze, saved.ID); err != nil {
		t.Fatalf("kaze deleting: %v", err)
	}
	if _, ok, err := repo.AddSavedSong(hana, "lrclib", "42"); err != nil || ok {
		t.Errorf("AddSavedSong after both deleted it = %v, %v, want it gone", ok, err)
	}
}

func TestSQLiteNeedsASignedInUserWithAccounts(t *testing.T) {
	t.Parallel()

	repo, _, _ := twoUsers(t)
	ctx := t.Context()
	calls := map[string]error{}
	_, calls["Songs"] = repo.Songs(ctx)
	_, calls["SongByID"] = repo.SongByID(ctx, 1)
	_, calls["AddSong"] = repo.AddSong(ctx, Song{Title: "x", Source: SourceUser, Lines: sampleLines()})
	_, _, calls["AddSavedSong"] = repo.AddSavedSong(ctx, "lrclib", "1")
	_, calls["SongIDsBySource"] = repo.SongIDsBySource(ctx, "lrclib", []string{"1"})
	calls["SaveAnalysis"] = repo.SaveAnalysis(ctx, 1, nil)
	calls["DeleteSong"] = repo.DeleteSong(ctx, 1)
	for name, err := range calls {
		if !errors.Is(err, fault.ErrUnauthorized) {
			t.Errorf("%s with no one signed in: %v, want unauthorized", name, err)
		}
	}
}
