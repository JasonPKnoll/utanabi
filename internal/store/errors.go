package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// Translate turns a driver error into the project's own errors, so nothing above the adapter sees a driver error.
// Call it on every error that comes back from database/sql.
func Translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return fault.ErrNotFound
	case errors.Is(err, context.Canceled):
		return fault.ErrCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return fault.ErrTimeout
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("store: duplicate row (%v); %w", err, fault.ErrConflict)
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			return fmt.Errorf("store: database is locked (%v); %w", err, fault.ErrTimeout)
		}
	}
	return fmt.Errorf("store: query failed (%v)", err)
}
