// Package storetest opens a fresh SQLite database file for each test.
// SQLite runs in process, so integration tests use the real database without a container.
package storetest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/store"
)

// New opens an empty database in the test's temp folder and closes it when the test ends.
func New(tb testing.TB) *store.DB {
	tb.Helper()
	db, err := store.Open(tb.Context(), filepath.Join(tb.TempDir(), "test.db"), 5*time.Second)
	if err != nil {
		tb.Fatalf("storetest: open database: %v", err)
	}
	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Errorf("storetest: close database: %v", err)
		}
	})
	return db
}
