package spotify

import (
	"context"
	"database/sql"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database. A link is its owner's; a song's track is shared, like the
// song, but only read or saved for a song in the caller's library.
type SQLite struct {
	db     *store.DB
	owners identity.Owners
}

func NewSQLite(db *store.DB, owners identity.Owners) *SQLite {
	return &SQLite{db: db, owners: owners}
}

func (s *SQLite) Link(ctx context.Context) (Link, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return Link{}, err
	}
	var access, refresh string
	var expires, connected int64
	var link Link
	err = s.db.Querier().QueryRowContext(ctx,
		`SELECT access_token, refresh_token, expires_at, display_name, connected_at FROM spotify_links WHERE owner = ?`,
		owner,
	).Scan(&access, &refresh, &expires, &link.DisplayName, &connected)
	if err != nil {
		return Link{}, store.Translate(err)
	}
	link.AccessToken, link.RefreshToken = redact.New(access), redact.New(refresh)
	link.ExpiresAt, link.ConnectedAt = time.UnixMilli(expires), time.UnixMilli(connected)
	return link, nil
}

func (s *SQLite) SaveLink(ctx context.Context, link Link) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.Querier().ExecContext(ctx,
		`INSERT INTO spotify_links (owner, access_token, refresh_token, expires_at, display_name, connected_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (owner) DO UPDATE SET access_token = excluded.access_token,
		   refresh_token = excluded.refresh_token, expires_at = excluded.expires_at,
		   display_name = excluded.display_name, connected_at = excluded.connected_at`,
		owner, link.AccessToken.Reveal(), link.RefreshToken.Reveal(), link.ExpiresAt.UnixMilli(), link.DisplayName,
		link.ConnectedAt.UnixMilli())
	return store.Translate(err)
}

func (s *SQLite) DeleteLink(ctx context.Context) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.Querier().ExecContext(ctx, `DELETE FROM spotify_links WHERE owner = ?`, owner)
	return store.Translate(err)
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

func (s *SQLite) TrackFor(ctx context.Context, songID int64) (string, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return "", err
	}
	var trackID string
	err = s.db.Querier().QueryRowContext(ctx,
		`SELECT t.track_id FROM spotify_tracks t
		 WHERE t.song_id = ? AND EXISTS (SELECT 1 FROM library_songs b WHERE b.owner = ? AND b.song_id = t.song_id)`,
		songID, owner,
	).Scan(&trackID)
	return trackID, store.Translate(err)
}

func (s *SQLite) SaveTrack(ctx context.Context, songID int64, trackID string, now time.Time) error {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return err
	}
	_, err = s.db.Querier().ExecContext(ctx,
		`INSERT INTO spotify_tracks (song_id, track_id, matched_at)
		 SELECT ?, ?, ? WHERE EXISTS (SELECT 1 FROM library_songs b WHERE b.owner = ? AND b.song_id = ?)
		 ON CONFLICT (song_id) DO UPDATE SET track_id = excluded.track_id, matched_at = excluded.matched_at`,
		songID, trackID, now.UnixMilli(), owner, songID)
	return store.Translate(err)
}
