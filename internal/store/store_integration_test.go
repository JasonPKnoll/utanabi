//go:build integration

package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

func createTable(t *testing.T, db *store.DB) {
	t.Helper()
	_, err := db.Querier().ExecContext(t.Context(), `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
}

func countItems(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Querier().QueryRowContext(t.Context(), `SELECT count(*) FROM items`).Scan(&n); err != nil {
		t.Fatalf("count items: %v", err)
	}
	return n
}

func insert(ctx context.Context, q store.Querier, name string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO items (name) VALUES (?)`, name)
	return store.Translate(err)
}

func TestOpenCreatesFileAndPings(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "app.db")
	db, err := store.Open(t.Context(), path, time.Second)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	if err := db.Ping(t.Context()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	var on int
	if err := db.Querier().QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&on); err != nil {
		t.Fatalf("read pragma: %v", err)
	}
	if on != 1 {
		t.Errorf("foreign_keys = %d, want 1", on)
	}
}

func TestWithTxCommits(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createTable(t, db)

	err := db.WithTx(t.Context(), func(q store.Querier) error {
		return insert(t.Context(), q, "first")
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if got, want := countItems(t, db), 1; got != want {
		t.Errorf("rows = %d, want %d", got, want)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createTable(t, db)
	errStop := errors.New("stop")

	err := db.WithTx(t.Context(), func(q store.Querier) error {
		if err := insert(t.Context(), q, "first"); err != nil {
			return err
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("WithTx() = %v, want %v", err, errStop)
	}
	if got := countItems(t, db); got != 0 {
		t.Errorf("rows = %d, want 0 after rollback", got)
	}
}

func TestWithTxRollsBackOnPanic(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createTable(t, db)

	func() {
		defer func() {
			if p := recover(); p == nil {
				t.Errorf("WithTx did not re-raise the panic")
			}
		}()
		_ = db.WithTx(t.Context(), func(q store.Querier) error {
			if err := insert(t.Context(), q, "first"); err != nil {
				t.Errorf("insert: %v", err)
			}
			panic("boom")
		})
	}()

	if got := countItems(t, db); got != 0 {
		t.Errorf("rows = %d, want 0 after rollback", got)
	}
}

func TestTranslateUniqueViolationIsConflict(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createTable(t, db)
	if err := insert(t.Context(), db.Querier(), "same"); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	err := insert(t.Context(), db.Querier(), "same")
	if !errors.Is(err, fault.ErrConflict) {
		t.Errorf("second insert = %v, want %v", err, fault.ErrConflict)
	}
}

func TestTranslateMissingRowIsNotFound(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	createTable(t, db)

	var name string
	err := db.Querier().QueryRowContext(t.Context(), `SELECT name FROM items WHERE id = 1`).Scan(&name)
	if got := store.Translate(err); !errors.Is(got, fault.ErrNotFound) {
		t.Errorf("Translate() = %v, want %v", got, fault.ErrNotFound)
	}
}
