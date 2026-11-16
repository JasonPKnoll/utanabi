//go:build integration

package store_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

func sqlFile(sql string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(sql)}
}

func twoMigrations() fstest.MapFS {
	return fstest.MapFS{
		"001_create_notes.sql": sqlFile(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL);`),
		"002_seed_note.sql":    sqlFile(`INSERT INTO notes (body) VALUES ('first');`),
	}
}

func userVersion(t *testing.T, db *store.DB) int {
	t.Helper()
	var v int
	if err := db.Querier().QueryRowContext(t.Context(), `PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

func countNotes(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Querier().QueryRowContext(t.Context(), `SELECT count(*) FROM notes`).Scan(&n); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	return n
}

func tableExists(t *testing.T, db *store.DB, name string) bool {
	t.Helper()
	var n int
	err := db.Querier().QueryRowContext(t.Context(),
		`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("look up table %s: %v", name, err)
	}
	return n == 1
}

func TestMigrateFreshDatabase(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if err := db.Migrate(t.Context(), twoMigrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if got, want := userVersion(t, db), 2; got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if got, want := countNotes(t, db), 1; got != want {
		t.Errorf("notes rows = %d, want %d", got, want)
	}
}

func TestMigrateUpToDateAppliesNothing(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if err := db.Migrate(t.Context(), twoMigrations()); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := db.Migrate(t.Context(), twoMigrations()); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if got, want := userVersion(t, db), 2; got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if got, want := countNotes(t, db), 1; got != want {
		t.Errorf("notes rows = %d, want %d, so a migration ran twice", got, want)
	}
}

func TestMigrateAppliesOnlyPendingFiles(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	first := fstest.MapFS{"001_create_notes.sql": twoMigrations()["001_create_notes.sql"]}
	if err := db.Migrate(t.Context(), first); err != nil {
		t.Fatalf("Migrate first file: %v", err)
	}
	if got, want := userVersion(t, db), 1; got != want {
		t.Fatalf("user_version = %d, want %d", got, want)
	}

	if err := db.Migrate(t.Context(), twoMigrations()); err != nil {
		t.Fatalf("Migrate both files: %v", err)
	}
	if got, want := userVersion(t, db), 2; got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if got, want := countNotes(t, db), 1; got != want {
		t.Errorf("notes rows = %d, want %d", got, want)
	}
}

func TestMigrateFailedFileStopsAtLastGoodVersion(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	fsys := twoMigrations()
	fsys["003_broken.sql"] = sqlFile(`CREATE TABLE tags (id INTEGER PRIMARY KEY); INSERT INTO missing VALUES (1);`)
	fsys["004_never.sql"] = sqlFile(`CREATE TABLE never (id INTEGER PRIMARY KEY);`)

	err := db.Migrate(t.Context(), fsys)
	if err == nil {
		t.Fatalf("Migrate() = nil, want an error")
	}
	if got := err.Error(); !strings.HasPrefix(got, "store: migration failed (003_broken.sql); store: query failed (") {
		t.Errorf("Migrate() = %q, want it to name 003_broken.sql and carry the driver detail", got)
	}
	if got, want := userVersion(t, db), 2; got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
	if tableExists(t, db, "tags") {
		t.Errorf("table tags exists, want the failed file rolled back")
	}
	if tableExists(t, db, "never") {
		t.Errorf("table never exists, want no file after the failed one applied")
	}
}

func TestMigrateRejectsNewerDatabase(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if _, err := db.Querier().ExecContext(t.Context(), `PRAGMA user_version = 5`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}

	err := db.Migrate(t.Context(), twoMigrations())
	if want := "store: database is newer than the app (version 5, latest known 2)"; err == nil || err.Error() != want {
		t.Errorf("Migrate() = %v, want %q", err, want)
	}
	if got, want := userVersion(t, db), 5; got != want {
		t.Errorf("user_version = %d, want %d", got, want)
	}
}

func TestMigrateEmbeddedMigrations(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if got := userVersion(t, db); got < 1 {
		t.Errorf("user_version = %d, want at least 1", got)
	}
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Errorf("second Migrate: %v", err)
	}
}
