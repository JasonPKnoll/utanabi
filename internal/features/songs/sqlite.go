package songs

import (
	"context"
	"database/sql"
	"strings"
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
			`INSERT INTO songs (title, artist, source, source_ref, duration_ms, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			song.Title, song.Artist, song.Source, nullString(song.SourceRef), nullPositive(song.DurationMS), now, now,
		).Scan(&song.ID)
		if err != nil {
			return store.Translate(err)
		}
		for _, line := range song.Lines {
			_, err := q.ExecContext(ctx,
				`INSERT INTO lyric_lines (song_id, position, stanza, text, start_ms) VALUES (?, ?, ?, ?, ?)`,
				song.ID, line.Position, line.Stanza, line.Text, line.StartMS,
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
	// One statement reads the song and its lines, so a delete can't land between them and leave a song with no lines.
	rows, err := s.db.Querier().QueryContext(ctx,
		`SELECT s.id, s.title, s.artist, s.source, s.source_ref, s.duration_ms,
		        l.position, l.stanza, l.text, l.start_ms
		 FROM songs s LEFT JOIN lyric_lines l ON l.song_id = s.id
		 WHERE s.id = ?
		 ORDER BY l.position`, id)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	defer rows.Close()

	var song Song
	found := false
	for rows.Next() {
		var ref sql.NullString
		var duration, position, stanza, start sql.NullInt64
		var text sql.NullString
		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Source, &ref, &duration,
			&position, &stanza, &text, &start)
		if err != nil {
			return Song{}, store.Translate(err)
		}
		if !found {
			found = true
			song.SourceRef, song.DurationMS = ref.String, duration.Int64
			song.Lines = []Line{}
		}
		if !position.Valid {
			continue
		}
		line := Line{Position: int(position.Int64), Stanza: int(stanza.Int64), Text: text.String}
		if start.Valid {
			line.StartMS = &start.Int64
		}
		song.Lines = append(song.Lines, line)
	}
	if err := rows.Err(); err != nil {
		return Song{}, store.Translate(err)
	}
	if !found {
		return Song{}, fault.ErrNotFound
	}
	return song, nil
}

// SongIDsBySource returns the library's song ID for each of refs already saved from source.
func (s *SQLite) SongIDsBySource(ctx context.Context, source string, refs []string) (map[string]int64, error) {
	found := map[string]int64{}
	if len(refs) == 0 {
		return found, nil
	}
	args := make([]any, 0, len(refs)+1)
	args = append(args, source)
	for _, ref := range refs {
		args = append(args, ref)
	}
	placeholders := strings.Repeat(", ?", len(refs))[2:]
	rows, err := s.db.Querier().QueryContext(ctx,
		`SELECT source_ref, id FROM songs WHERE source = ? AND source_ref IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref string
		var id int64
		if err := rows.Scan(&ref, &id); err != nil {
			return nil, store.Translate(err)
		}
		found[ref] = id
	}
	if err := rows.Err(); err != nil {
		return nil, store.Translate(err)
	}
	return found, nil
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

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullPositive(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: n > 0}
}
