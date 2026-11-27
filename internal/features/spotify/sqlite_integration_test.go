//go:build integration

package spotify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

// twoUsers is a repository in server mode with two users, and a song in hana's library only.
func twoUsers(t *testing.T) (*SQLite, context.Context, context.Context) {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, err := db.Querier().ExecContext(t.Context(), `
		INSERT INTO users (id, handle, name, created_at) VALUES (1, randomblob(32), 'hana', 1), (2, randomblob(32), 'kaze', 1);
		INSERT INTO songs (id, title, artist, source, source_ref, duration_ms, created_at, updated_at)
			VALUES (10, 'Shinunoga E-Wa', 'Fujii Kaze', 'lrclib', '1', 185000, 1, 1);
		INSERT INTO library_songs (owner, song_id, added_at) VALUES (1, 10, 1);`)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	hana := identity.Into(t.Context(), identity.User{ID: 1, Name: "hana"})
	kaze := identity.Into(t.Context(), identity.User{ID: 2, Name: "kaze"})
	return NewSQLite(db, identity.Accounts()), hana, kaze
}

func link(access string) Link {
	return Link{AccessToken: redact.New(access), RefreshToken: redact.New("refresh-" + access),
		ExpiresAt: time.UnixMilli(1_800_000_000_000), DisplayName: "Hana", ConnectedAt: time.UnixMilli(1_700_000_000_000)}
}

func TestSQLiteKeepsEachOwnersLink(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	if _, err := repo.Link(hana); !errors.Is(err, fault.ErrNotFound) {
		t.Fatalf("before connecting: %v, want not found", err)
	}
	if err := repo.SaveLink(hana, link("a1")); err != nil {
		t.Fatalf("SaveLink: %v", err)
	}
	if err := repo.SaveLink(hana, link("a2")); err != nil {
		t.Fatalf("SaveLink again: %v", err)
	}
	got, err := repo.Link(hana)
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if got.AccessToken.Reveal() != "a2" || got.RefreshToken.Reveal() != "refresh-a2" || got.DisplayName != "Hana" ||
		!got.ExpiresAt.Equal(time.UnixMilli(1_800_000_000_000)) || !got.ConnectedAt.Equal(time.UnixMilli(1_700_000_000_000)) {
		t.Errorf("got %+v, want the second link", got)
	}
	if _, err := repo.Link(kaze); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze sees hana's link: %v", err)
	}
	if err := repo.DeleteLink(kaze); err != nil {
		t.Fatalf("kaze DeleteLink: %v", err)
	}
	if _, err := repo.Link(hana); err != nil {
		t.Errorf("kaze disconnecting removed hana's link: %v", err)
	}
	if err := repo.DeleteLink(hana); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if _, err := repo.Link(hana); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("after disconnecting: %v, want not found", err)
	}
}

func TestSQLiteFindsSongsAndTracksInTheOwnersLibraryOnly(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	song, err := repo.Song(hana, 10)
	if err != nil || song != (Song{Title: "Shinunoga E-Wa", Artist: "Fujii Kaze", DurationMS: 185000}) {
		t.Errorf("Song = %+v, %v", song, err)
	}
	if _, err := repo.Song(kaze, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze reading hana's song: %v, want not found", err)
	}

	if _, err := repo.TrackFor(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("before matching: %v, want not found", err)
	}
	if err := repo.SaveTrack(kaze, 10, Track{ID: kazeTrack}, time.UnixMilli(1)); err != nil {
		t.Fatalf("kaze SaveTrack: %v", err)
	}
	if _, err := repo.TrackFor(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Error("kaze saved a track for a song not in her library")
	}
	if err := repo.SaveTrack(hana, 10, Track{ID: kazeTrack}, time.UnixMilli(1)); err != nil {
		t.Fatalf("SaveTrack: %v", err)
	}
	if got, err := repo.TrackFor(hana, 10); err != nil || got.ID != kazeTrack || got.DurationMS != 0 {
		t.Errorf("TrackFor = %+v, %v; want the match with no length", got, err)
	}
	if err := repo.SaveTrack(hana, 10, Track{ID: "2222222222222222222222", DurationMS: 179500},
		time.UnixMilli(2)); err != nil {
		t.Fatalf("SaveTrack again: %v", err)
	}
	if got, err := repo.TrackFor(hana, 10); err != nil || got.ID != "2222222222222222222222" ||
		got.DurationMS != 179500 {
		t.Errorf("TrackFor = %+v, %v; want the newer match with its length", got, err)
	}
	if _, err := repo.TrackFor(kaze, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze reading the track of a song not in her library: %v", err)
	}
}

func TestSQLiteNeedsASignedInUserWithAccounts(t *testing.T) {
	t.Parallel()

	repo, _, _ := twoUsers(t)
	ctx := t.Context()
	calls := map[string]error{}
	_, calls["Link"] = repo.Link(ctx)
	calls["SaveLink"] = repo.SaveLink(ctx, link("a1"))
	calls["DeleteLink"] = repo.DeleteLink(ctx)
	_, calls["Song"] = repo.Song(ctx, 10)
	_, calls["TrackFor"] = repo.TrackFor(ctx, 10)
	calls["SaveTrack"] = repo.SaveTrack(ctx, 10, Track{ID: kazeTrack}, time.UnixMilli(1))
	_, calls["Offset"] = repo.Offset(ctx, 10)
	calls["SaveOffset"] = repo.SaveOffset(ctx, 10, 1000)
	calls["DeleteOffset"] = repo.DeleteOffset(ctx, 10)
	for name, err := range calls {
		if !errors.Is(err, fault.ErrUnauthorized) {
			t.Errorf("%s with no one signed in: %v, want unauthorized", name, err)
		}
	}
}

func TestSQLiteKeepsEachOwnersOffset(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	if _, err := repo.Offset(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("before setting one: %v, want not found", err)
	}
	if err := repo.SaveOffset(hana, 10, -5500); err != nil {
		t.Fatalf("SaveOffset: %v", err)
	}
	if err := repo.SaveOffset(hana, 10, -6000); err != nil {
		t.Fatalf("SaveOffset again: %v", err)
	}
	if got, err := repo.Offset(hana, 10); err != nil || got != -6000 {
		t.Errorf("Offset = %d, %v; want the newer one, -6000", got, err)
	}
	if err := repo.SaveOffset(kaze, 10, 2000); err != nil {
		t.Fatalf("kaze SaveOffset: %v", err)
	}
	if _, err := repo.Offset(kaze, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Error("kaze saved an offset for a song not in her library")
	}
	if err := repo.DeleteOffset(hana, 10); err != nil {
		t.Fatalf("DeleteOffset: %v", err)
	}
	if _, err := repo.Offset(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("after DeleteOffset: %v, want not found", err)
	}
	if err := repo.DeleteOffset(hana, 10); err != nil {
		t.Errorf("DeleteOffset with none set: %v", err)
	}
}
