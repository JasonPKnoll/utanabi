// Package store opens the local SQLite database, runs transactions, and translates driver errors.
// Slices talk to the database only through their sqlite.go file, using a Querier.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Querier runs SQL. Both *sql.DB and *sql.Tx satisfy it, so a query function works inside or outside a transaction.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DB is the app's database handle.
type DB struct {
	sql *sql.DB
}

// Open creates the database file and its folder if needed, and checks the connection.
// busyTimeout is how long a statement waits for a lock before it fails.
func Open(ctx context.Context, path string, busyTimeout time.Duration) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("store: create data folder failed (%v)", err)
	}

	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeout.Milliseconds()))
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	// Write transactions take the lock up front, so two writers wait on busy_timeout instead of deadlocking.
	q.Add("_txlock", "immediate")
	dsn := (&url.URL{Scheme: "file", Opaque: path, RawQuery: q.Encode()}).String()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open database failed (%v)", err)
	}
	if err := db.PingContext(ctx); err != nil {
		closeErr := db.Close()
		return nil, errors.Join(Translate(err), closeErr)
	}
	return &DB{sql: db}, nil
}

// Querier returns the handle for queries outside a transaction.
func (db *DB) Querier() Querier {
	return db.sql
}

// Ping checks that the database answers.
func (db *DB) Ping(ctx context.Context) error {
	return Translate(db.sql.PingContext(ctx))
}

// Close closes the database. Call it once, at shutdown.
func (db *DB) Close() error {
	if err := db.sql.Close(); err != nil {
		return fmt.Errorf("store: close database failed (%v)", err)
	}
	return nil
}

// WithTx runs fn in one transaction. It commits when fn returns nil, and rolls back on an error or a panic.
func (db *DB) WithTx(ctx context.Context, fn func(q Querier) error) (err error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Translate(err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // the panic is the failure that matters, and it is re-raised below
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = errors.Join(err, Translate(rbErr))
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return Translate(tx.Commit())
}
