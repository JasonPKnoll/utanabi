package store

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var embedded embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// Migrations returns the app's schema migrations, the numbered .sql files in internal/store/migrations.
func Migrations() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(fmt.Sprintf("store: embedded migrations folder is missing (%v)", err))
	}
	return sub
}

// Migrate applies the .sql files at the root of migrations that the database has not seen yet, in version order.
// Files are named <version>_<name>.sql, with versions 1, 2, 3 and so on, no gaps. Each file runs in its own
// transaction, which also sets PRAGMA user_version to the file's version, so a failed file leaves the database at
// the last good one.
func (db *DB) Migrate(ctx context.Context, migrations fs.FS) error {
	all, err := loadMigrations(migrations)
	if err != nil {
		return err
	}

	current, err := userVersion(ctx, db.sql)
	if err != nil {
		return err
	}
	if current > len(all) {
		return fmt.Errorf("store: database is newer than the app (version %d, latest known %d)", current, len(all))
	}

	for _, m := range all[current:] {
		if err := db.WithTx(ctx, func(q Querier) error { return apply(ctx, q, m) }); err != nil {
			return fmt.Errorf("store: migration failed (%s); %w", m.name, err)
		}
	}
	return nil
}

func apply(ctx context.Context, q Querier, m migration) error {
	// Another process may have applied this file since the version was read outside the transaction.
	current, err := userVersion(ctx, q)
	if err != nil {
		return err
	}
	if current >= m.version {
		return nil
	}
	if _, err := q.ExecContext(ctx, m.sql); err != nil {
		return Translate(err)
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.version))
	return Translate(err)
}

func userVersion(ctx context.Context, q Querier) (int, error) {
	var v int
	if err := q.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return 0, Translate(err)
	}
	return v, nil
}

func loadMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("store: list migrations failed (%v)", err)
	}

	all := make([]migration, 0, len(names))
	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version <= 0 {
			return nil, fmt.Errorf("store: migration name must start with a positive version and an underscore (%s)", name)
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("store: read migration failed (%v)", err)
		}
		all = append(all, migration{version: version, name: name, sql: string(body)})
	}

	slices.SortStableFunc(all, func(a, b migration) int { return cmp.Compare(a.version, b.version) })
	for i, m := range all {
		if m.version != i+1 {
			return nil, fmt.Errorf("store: migration versions must run 1, 2, 3 with no gaps or repeats (%s)", m.name)
		}
	}
	return all, nil
}
