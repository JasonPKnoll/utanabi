package youtube

import (
	"context"
	"database/sql"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database. Every video is its owner's, and only for a song in their
// library.
type SQLite struct {
	db     *store.DB
	owners identity.Owners
}

func NewSQLite(db *store.DB, owners identity.Owners) *SQLite {
	return &SQLite{db: db, owners: owners}
}

func (s *SQLite) Song(ctx context.Context, songID int64) (Song, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return Song{}, err
	}
	var song Song
	var duration sql.NullInt64
	err = s.db.Querier().QueryRowContext(ctx,
		`SELECT s.title, s.artist, s.duration_ms FROM songs s
		 WHERE s.id = ? AND EXISTS (SELECT 1 FROM library_songs b WHERE b.owner = ? AND b.song_id = s.id)`,
		songID, owner,
	).Scan(&song.Title, &song.Artist, &duration)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	song.DurationMS = duration.Int64
	return song, nil
}

func (s *SQLite) VideoFor(ctx context.Context, songID int64) (Video, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return Video{}, err
	}
	var v Video
	var duration sql.NullInt64
	err = s.db.Querier().QueryRowContext(ctx,
		`SELECT v.video_id, v.title, v.channel, v.duration_ms, v.offset_ms, v.chosen FROM youtube_videos v
		 WHERE v.owner = ? AND v.song_id = ?
		   AND EXISTS (SELECT 1 FROM library_songs b WHERE b.owner = v.owner AND b.song_id = v.song_id)`,
		owner, songID,
	).Scan(&v.ID, &v.Title, &v.Channel, &duration, &v.OffsetMS, &v.Chosen)
	if err != nil {
		return Video{}, store.Translate(err)
	}
	v.DurationMS = duration.Int64
	return v, nil
}

func (s *SQLite) SaveVideo(ctx context.Context, songID int64, video Video, now time.Time) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	var duration sql.NullInt64
	if video.DurationMS > 0 {
		duration = sql.NullInt64{Int64: video.DurationMS, Valid: true}
	}
	_, err = s.db.Querier().ExecContext(ctx,
		`INSERT INTO youtube_videos (owner, song_id, video_id, title, channel, duration_ms, offset_ms, chosen, matched_at)
		 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?
		 WHERE EXISTS (SELECT 1 FROM library_songs b WHERE b.owner = ? AND b.song_id = ?)
		 ON CONFLICT (owner, song_id) DO UPDATE SET video_id = excluded.video_id, title = excluded.title,
		   channel = excluded.channel, duration_ms = excluded.duration_ms, offset_ms = excluded.offset_ms,
		   chosen = excluded.chosen, matched_at = excluded.matched_at`,
		owner, songID, video.ID, video.Title, video.Channel, duration, video.OffsetMS, video.Chosen, now.UnixMilli(),
		owner, songID)
	return store.Translate(err)
}

func (s *SQLite) SaveOffset(ctx context.Context, songID, offsetMS int64) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	res, err := s.db.Querier().ExecContext(ctx,
		`UPDATE youtube_videos SET offset_ms = ? WHERE owner = ? AND song_id = ?`, offsetMS, owner, songID)
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

func (s *SQLite) DeleteVideo(ctx context.Context, songID int64) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.Querier().ExecContext(ctx,
		`DELETE FROM youtube_videos WHERE owner = ? AND song_id = ?`, owner, songID)
	return store.Translate(err)
}
