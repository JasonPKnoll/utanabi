package songs

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database.
type SQLite struct {
	db *store.DB
}

// NewSQLite returns a Repository that stores songs in db.
func NewSQLite(db *store.DB) *SQLite {
	return &SQLite{db: db}
}

// AddSong inserts the song and its lines in one transaction, and returns the song with its new ID.
func (s *SQLite) AddSong(ctx context.Context, song Song) (Song, error) {
	now := time.Now().UnixMilli()
	err := s.db.WithTx(ctx, func(q store.Querier) error {
		err := q.QueryRowContext(ctx,
			`INSERT INTO songs (title, artist, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
			song.Title, song.Artist, song.Source, now, now,
		).Scan(&song.ID)
		if err != nil {
			return store.Translate(err)
		}
		for _, line := range song.Lines {
			_, err := q.ExecContext(ctx,
				`INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (?, ?, ?, ?)`,
				song.ID, line.Position, line.Stanza, line.Text,
			)
			if err != nil {
				return store.Translate(err)
			}
		}
		return nil
	})
	if err != nil {
		return Song{}, err
	}
	return song, nil
}

// Songs returns every song without its lines, newest first.
func (s *SQLite) Songs(ctx context.Context) ([]Summary, error) {
	rows, err := s.db.Querier().QueryContext(ctx, `SELECT id, title, artist, source FROM songs ORDER BY id DESC`)
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

// SongByID returns one song with its lines in order, or fault.ErrNotFound.
func (s *SQLite) SongByID(ctx context.Context, id int64) (Song, error) {
	q := s.db.Querier()
	var song Song
	err := q.QueryRowContext(ctx, `SELECT id, title, artist, source FROM songs WHERE id = ?`, id).
		Scan(&song.ID, &song.Title, &song.Artist, &song.Source)
	if err != nil {
		return Song{}, store.Translate(err)
	}

	rows, err := q.QueryContext(ctx,
		`SELECT position, stanza, text FROM lyric_lines WHERE song_id = ? ORDER BY position`, id)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	defer rows.Close()

	song.Lines = []Line{}
	for rows.Next() {
		var line Line
		if err := rows.Scan(&line.Position, &line.Stanza, &line.Text); err != nil {
			return Song{}, store.Translate(err)
		}
		song.Lines = append(song.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return Song{}, store.Translate(err)
	}
	return song, nil
}

// DeleteSong removes one song; its lines and everything learned from them go with it. It returns fault.ErrNotFound
// if there is no such song.
func (s *SQLite) DeleteSong(ctx context.Context, id int64) error {
	res, err := s.db.Querier().ExecContext(ctx, `DELETE FROM songs WHERE id = ?`, id)
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
