package songs

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database.
type SQLite struct {
	q store.Querier
}

// NewSQLite returns a Repository that runs its queries on q.
func NewSQLite(q store.Querier) *SQLite {
	return &SQLite{q: q}
}

// AddSong inserts the song and returns it with its new ID.
func (s *SQLite) AddSong(ctx context.Context, song Song) (Song, error) {
	err := s.q.QueryRowContext(ctx,
		`INSERT INTO songs (title, artist, lyrics, source, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		song.Title, song.Artist, song.Lyrics, song.Source, time.Now().UnixMilli(),
	).Scan(&song.ID)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	return song, nil
}

// Songs returns every song without its lyrics, newest first.
func (s *SQLite) Songs(ctx context.Context) ([]Summary, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT id, title, artist, source FROM songs ORDER BY id DESC`)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()

	list := []Summary{}
	for rows.Next() {
		var sum Summary
		if err := rows.Scan(&sum.ID, &sum.Title, &sum.Artist, &sum.Source); err != nil {
			return nil, store.Translate(err)
		}
		list = append(list, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, store.Translate(err)
	}
	return list, nil
}

// SongByID returns one song with its lyrics, or fault.ErrNotFound.
func (s *SQLite) SongByID(ctx context.Context, id int64) (Song, error) {
	var song Song
	err := s.q.QueryRowContext(ctx,
		`SELECT id, title, artist, lyrics, source FROM songs WHERE id = ?`, id,
	).Scan(&song.ID, &song.Title, &song.Artist, &song.Lyrics, &song.Source)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	return song, nil
}

// DeleteSong removes one song, or returns fault.ErrNotFound if there is no such song.
func (s *SQLite) DeleteSong(ctx context.Context, id int64) error {
	res, err := s.q.ExecContext(ctx, `DELETE FROM songs WHERE id = ?`, id)
	if err != nil {
		return store.Translate(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return store.Translate(err)
	}
	if n == 0 {
		return fault.ErrNotFound
	}
	return nil
}
