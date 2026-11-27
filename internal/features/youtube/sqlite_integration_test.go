//go:build integration

package youtube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

// twoUsers is a repository in server mode with two users and a song in both libraries.
func twoUsers(t *testing.T) (*SQLite, context.Context, context.Context) {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	_, err := db.Querier().ExecContext(t.Context(), `
		INSERT INTO users (id, handle, name, created_at) VALUES (1, randomblob(32), 'hana', 1), (2, randomblob(32), 'kaze', 1);
		INSERT INTO songs (id, title, artist, source, source_ref, duration_ms, created_at, updated_at)
			VALUES (10, 'Matsuri', 'Fujii Kaze', 'lrclib', '1', 249000, 1, 1),
			       (11, 'Hana', 'Fujii Kaze', 'lrclib', '2', NULL, 1, 1);
		INSERT INTO library_songs (owner, song_id, added_at) VALUES (1, 10, 1), (2, 10, 1), (1, 11, 1);`)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	hana := identity.Into(t.Context(), identity.User{ID: 1, Name: "hana"})
	kaze := identity.Into(t.Context(), identity.User{ID: 2, Name: "kaze"})
	return NewSQLite(db, identity.Accounts()), hana, kaze
}

func TestSQLiteSongInTheOwnersLibrary(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	if got, err := repo.Song(hana, 10); err != nil || got != (Song{Title: "Matsuri", Artist: "Fujii Kaze",
		DurationMS: 249000}) {
		t.Errorf("Song = %+v, %v", got, err)
	}
	if got, err := repo.Song(hana, 11); err != nil || got.DurationMS != 0 {
		t.Errorf("a song with no length = %+v, %v", got, err)
	}
	if _, err := repo.Song(kaze, 11); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze reading hana's song: %v, want not found", err)
	}
}

func TestSQLiteKeepsEachOwnersVideo(t *testing.T) {
	t.Parallel()

	repo, hana, kaze := twoUsers(t)
	if _, err := repo.VideoFor(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("before one is saved: %v, want not found", err)
	}
	official := Video{ID: "BBBBBBBBBBB", Title: "Matsuri (Official Video)", Channel: "Fujii Kaze", DurationMS: 249000}
	if err := repo.SaveVideo(hana, 10, official, time.UnixMilli(1)); err != nil {
		t.Fatalf("SaveVideo: %v", err)
	}
	if got, err := repo.VideoFor(hana, 10); err != nil || got != official {
		t.Errorf("VideoFor = %+v, %v; want %+v", got, err, official)
	}
	if _, err := repo.VideoFor(kaze, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Error("kaze sees hana's video for a song they both have")
	}

	if err := repo.SaveOffset(hana, 10, -900); err != nil {
		t.Fatalf("SaveOffset: %v", err)
	}
	if err := repo.SaveOffset(kaze, 10, 500); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze's offset with no video of their own: %v, want not found", err)
	}
	pasted := Video{ID: "dQw4w9WgXcQ", Chosen: true}
	if err := repo.SaveVideo(hana, 10, pasted, time.UnixMilli(2)); err != nil {
		t.Fatalf("SaveVideo again: %v", err)
	}
	if got, err := repo.VideoFor(hana, 10); err != nil || got != pasted {
		t.Errorf("after pasting: %+v, %v; want %+v, with no title, length, or offset", got, err, pasted)
	}

	if err := repo.SaveVideo(kaze, 11, official, time.UnixMilli(1)); err != nil {
		t.Fatalf("kaze SaveVideo: %v", err)
	}
	if _, err := repo.VideoFor(kaze, 11); !errors.Is(err, fault.ErrNotFound) {
		t.Error("kaze saved a video for a song not in their library")
	}

	if err := repo.DeleteVideo(hana, 10); err != nil {
		t.Fatalf("DeleteVideo: %v", err)
	}
	if _, err := repo.VideoFor(hana, 10); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("after DeleteVideo: %v, want not found", err)
	}
}

func TestSQLiteNeedsASignedInUserWithAccounts(t *testing.T) {
	t.Parallel()

	repo, _, _ := twoUsers(t)
	ctx := t.Context()
	calls := map[string]error{}
	_, calls["Song"] = repo.Song(ctx, 10)
	_, calls["VideoFor"] = repo.VideoFor(ctx, 10)
	calls["SaveVideo"] = repo.SaveVideo(ctx, 10, Video{ID: "BBBBBBBBBBB"}, time.UnixMilli(1))
	calls["SaveOffset"] = repo.SaveOffset(ctx, 10, 1)
	calls["DeleteVideo"] = repo.DeleteVideo(ctx, 10)
	for name, err := range calls {
		if !errors.Is(err, fault.ErrUnauthorized) {
			t.Errorf("%s with no one signed in: %v, want unauthorized", name, err)
		}
	}
}
