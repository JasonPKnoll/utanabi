package study

import (
	"context"
	"database/sql"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database. Its songs are the caller's library, and its cards the
// caller's own: owners says whose those are.
type SQLite struct {
	db     *store.DB
	owners identity.Owners
}

// NewSQLite returns a Repository that keeps cards and their progress in db, for the owners owners picks.
func NewSQLite(db *store.DB, owners identity.Owners) *SQLite {
	return &SQLite{db: db, owners: owners}
}

// SongWords returns the song's title and its content words that point at a dictionary meaning, in the order they're sung, with
// where each one sits in its line. A missing song is fault.ErrNotFound, through store.Translate.
func (s *SQLite) SongWords(ctx context.Context, songID int64) (string, []SongWord, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return "", nil, err
	}
	q := s.db.Querier()
	var title string
	err = q.QueryRowContext(ctx,
		`SELECT s.title FROM songs s JOIN library_songs l ON l.song_id = s.id WHERE s.id = ? AND l.owner = ?`,
		songID, owner).Scan(&title)
	if err != nil {
		return "", nil, store.Translate(err)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT w.entry_id, w.sense_index, w.lemma, coalesce(w.lemma_reading, ''), l.id, l.text,
			(SELECT coalesce(sum(length(t.surface)), 0) FROM line_tokens AS t
			 WHERE t.line_id = l.id AND t.position < w.first_token),
			(SELECT group_concat(t.surface, '' ORDER BY t.position) FROM line_tokens AS t
			 WHERE t.line_id = l.id AND t.position BETWEEN w.first_token AND w.last_token)
		FROM line_words AS w
		JOIN lyric_lines AS l ON l.id = w.line_id
		WHERE l.song_id = ? AND w.kind = 'content' AND w.entry_id IS NOT NULL AND w.sense_index IS NOT NULL
		ORDER BY l.position, w.first_token`, songID)
	if err != nil {
		return "", nil, store.Translate(err)
	}
	defer rows.Close()

	var words []SongWord
	for rows.Next() {
		var w SongWord
		var text sql.NullString
		if err := rows.Scan(&w.EntryID, &w.SenseIndex, &w.Lemma, &w.LemmaReading, &w.LineID, &w.LineText,
			&w.WordStart, &text); err != nil {
			return "", nil, store.Translate(err)
		}
		w.WordText = text.String
		words = append(words, w)
	}
	return title, words, store.Translate(rows.Err())
}

// WordCards saves a word card for each word that has none, from the line it's first sung in, and returns every
// word's card with its progress, in order.
func (s *SQLite) WordCards(ctx context.Context, words []SongWord, now time.Time) ([]StoredCard, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return nil, err
	}
	cards := make([]StoredCard, 0, len(words))
	err = s.db.WithTx(ctx, func(q store.Querier) error {
		cards = cards[:0]
		for _, w := range words {
			_, err := q.ExecContext(ctx, `
				INSERT INTO cards (kind, owner, entry_id, sense_index, context_line_id, label, label_reading, created_at)
				VALUES ('word', ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (owner, entry_id, sense_index) WHERE kind = 'word' DO NOTHING`,
				owner, w.EntryID, w.SenseIndex, w.LineID, w.Lemma,
				sql.NullString{String: w.LemmaReading, Valid: w.LemmaReading != ""}, now.UnixMilli())
			if err != nil {
				return store.Translate(err)
			}
			c := StoredCard{EntryID: w.EntryID, SenseIndex: w.SenseIndex}
			var reading sql.NullString
			err = q.QueryRowContext(ctx, `
				SELECT c.id, c.label, c.label_reading,
					coalesce(p.streak, 0), coalesce(p.right_count, 0), coalesce(p.wrong_count, 0)
				FROM cards AS c
				LEFT JOIN card_progress AS p ON p.card_id = c.id
				WHERE c.kind = 'word' AND c.owner = ? AND c.entry_id = ? AND c.sense_index = ?`,
				owner, w.EntryID, w.SenseIndex,
			).Scan(&c.ID, &c.Label, &reading, &c.Progress.Streak, &c.Progress.Right, &c.Progress.Wrong)
			if err != nil {
				return store.Translate(err)
			}
			c.LabelReading = reading.String
			cards = append(cards, c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

// RecordAnswer adds one answer to the card's progress: a right one lengthens its streak, a wrong one ends it.
func (s *SQLite) RecordAnswer(ctx context.Context, cardID int64, right bool, now time.Time) (Progress, error) {
	owner, err := s.owners.Of(ctx)
	if err != nil {
		return Progress{}, err
	}
	var p Progress
	err = s.db.WithTx(ctx, func(q store.Querier) error {
		var exists int
		// Someone else's card is not found.
		err := q.QueryRowContext(ctx, `SELECT 1 FROM cards WHERE id = ? AND owner = ?`, cardID, owner).Scan(&exists)
		if err != nil {
			return store.Translate(err)
		}
		r := 0
		if right {
			r = 1
		}
		err = q.QueryRowContext(ctx, `
			INSERT INTO card_progress (card_id, streak, right_count, wrong_count, reviewed_at)
			VALUES (?1, ?2, ?2, 1 - ?2, ?3)
			ON CONFLICT (card_id) DO UPDATE SET
				streak = CASE WHEN ?2 = 1 THEN streak + 1 ELSE 0 END,
				right_count = right_count + ?2,
				wrong_count = wrong_count + 1 - ?2,
				reviewed_at = ?3
			RETURNING streak, right_count, wrong_count`,
			cardID, r, now.UnixMilli(),
		).Scan(&p.Streak, &p.Right, &p.Wrong)
		return store.Translate(err)
	})
	if err != nil {
		return Progress{}, err
	}
	return p, nil
}
