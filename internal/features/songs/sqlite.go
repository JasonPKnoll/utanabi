package songs

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
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

// AddSong inserts the song, its lines, and their tokens in one transaction, and returns the song with its new ID.
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
			var lineID int64
			err := q.QueryRowContext(ctx,
				`INSERT INTO lyric_lines (song_id, position, stanza, text, start_ms) VALUES (?, ?, ?, ?, ?) RETURNING id`,
				song.ID, line.Position, line.Stanza, line.Text, line.StartMS,
			).Scan(&lineID)
			if err != nil {
				return store.Translate(err)
			}
			if err := insertTokens(ctx, q, lineID, line.Tokens); err != nil {
				return err
			}
			if err := insertWords(ctx, q, lineID, line.Analysis); err != nil {
				return err
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

// SongByID returns one song with its lines and their tokens in order, or fault.ErrNotFound.
func (s *SQLite) SongByID(ctx context.Context, id int64) (Song, error) {
	// One statement reads the song, its lines, and their tokens, so a delete can't land between them and leave a song
	// with no lines. Each row is one token, or one line when it has no tokens.
	rows, err := s.db.Querier().QueryContext(ctx,
		`SELECT s.id, s.title, s.artist, s.source, s.source_ref, s.duration_ms,
		        l.position, l.stanza, l.text, l.start_ms, t.surface, t.reading, t.pos
		 FROM songs s
		 LEFT JOIN lyric_lines l ON l.song_id = s.id
		 LEFT JOIN line_tokens t ON t.line_id = l.id
		 WHERE s.id = ?
		 ORDER BY l.position, t.position`, id)
	if err != nil {
		return Song{}, store.Translate(err)
	}
	defer rows.Close()

	var song Song
	found := false
	for rows.Next() {
		var ref, text, surface, reading, pos sql.NullString
		var duration, position, stanza, start sql.NullInt64
		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Source, &ref, &duration,
			&position, &stanza, &text, &start, &surface, &reading, &pos)
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
		n := len(song.Lines)
		if n == 0 || song.Lines[n-1].Position != int(position.Int64) {
			line := Line{Position: int(position.Int64), Stanza: int(stanza.Int64), Text: text.String}
			if start.Valid {
				line.StartMS = &start.Int64
			}
			song.Lines = append(song.Lines, line)
			n++
		}
		if surface.Valid {
			song.Lines[n-1].Tokens = append(song.Lines[n-1].Tokens,
				Token{Surface: surface.String, Reading: reading.String, POS: morph.POS(pos.String)})
		}
	}
	if err := rows.Err(); err != nil {
		return Song{}, store.Translate(err)
	}
	if !found {
		return Song{}, fault.ErrNotFound
	}
	if err := s.addWords(ctx, &song); err != nil {
		return Song{}, err
	}
	return song, nil
}

// addWords reads the words of a song's lines, with their conjugation steps in order. A line with no words keeps a
// nil Analysis.
func (s *SQLite) addWords(ctx context.Context, song *Song) error {
	rows, err := s.db.Querier().QueryContext(ctx,
		`SELECT l.position, w.id, w.first_token, w.last_token, w.kind, w.lemma, w.lemma_reading, w.conj_class,
		        w.entry_id, w.sense_index, w.grammar_point_id, w.analysis_source, i.grammar_point_id
		 FROM lyric_lines l
		 JOIN line_words w ON w.line_id = l.id
		 LEFT JOIN word_inflections i ON i.word_id = w.id
		 WHERE l.song_id = ?
		 ORDER BY l.position, w.first_token, i.step`, song.ID)
	if err != nil {
		return store.Translate(err)
	}
	defer rows.Close()

	byPosition := make(map[int]int, len(song.Lines))
	for i, line := range song.Lines {
		byPosition[line.Position] = i
	}
	for rows.Next() {
		var (
			position                      int
			w                             WordAnalysis
			kind                          string
			reading, class, grammar, step sql.NullString
			entry, sense                  sql.NullInt64
		)
		err := rows.Scan(&position, &w.ID, &w.FirstToken, &w.LastToken, &kind, &w.Lemma, &reading, &class,
			&entry, &sense, &grammar, &w.Source, &step)
		if err != nil {
			return store.Translate(err)
		}
		i, ok := byPosition[position]
		if !ok {
			continue
		}
		line := &song.Lines[i]
		if n := len(line.Analysis); n == 0 || line.Analysis[n-1].ID != w.ID {
			w.Kind, w.LemmaReading, w.ConjClass, w.GrammarPointID = WordKind(kind), reading.String, class.String,
				grammar.String
			w.EntryID, w.SenseIndex, w.head = entry.Int64, int(sense.Int64), -1
			line.Analysis = append(line.Analysis, w)
		}
		if step.Valid {
			last := &line.Analysis[len(line.Analysis)-1]
			last.Steps = append(last.Steps, step.String)
		}
	}
	return store.Translate(rows.Err())
}

// SaveAnalysis stores the tokens and words of each of lines that has none yet, matching lines by position, in one
// transaction. Lines that already have tokens keep them, and lines that already have words keep them, so two calls
// at once, a reading you corrected, or a word you corrected, are never overwritten. It returns fault.ErrNotFound if
// a line isn't there.
func (s *SQLite) SaveAnalysis(ctx context.Context, songID int64, lines []Line) error {
	return s.db.WithTx(ctx, func(q store.Querier) error {
		for _, line := range lines {
			if len(line.Tokens) == 0 {
				continue
			}
			var lineID int64
			var hasTokens, hasWords bool
			err := q.QueryRowContext(ctx,
				`SELECT l.id, EXISTS (SELECT 1 FROM line_tokens t WHERE t.line_id = l.id),
				        EXISTS (SELECT 1 FROM line_words w WHERE w.line_id = l.id)
				 FROM lyric_lines l WHERE l.song_id = ? AND l.position = ?`,
				songID, line.Position,
			).Scan(&lineID, &hasTokens, &hasWords)
			if err != nil {
				return store.Translate(err)
			}
			if !hasTokens {
				if err := insertTokens(ctx, q, lineID, line.Tokens); err != nil {
					return err
				}
			}
			if !hasWords {
				if err := insertWords(ctx, q, lineID, line.Analysis); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func insertWords(ctx context.Context, q store.Querier, lineID int64, words []WordAnalysis) error {
	for _, w := range words {
		var entry, sense sql.NullInt64
		if w.EntryID != 0 {
			entry = sql.NullInt64{Int64: w.EntryID, Valid: true}
			sense = sql.NullInt64{Int64: int64(w.SenseIndex), Valid: true}
		}
		source := w.Source
		if source == "" {
			source = AnalysisAuto
		}
		var wordID int64
		err := q.QueryRowContext(ctx,
			`INSERT INTO line_words (line_id, first_token, last_token, kind, lemma, lemma_reading, conj_class,
			                         entry_id, sense_index, grammar_point_id, analysis_source)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			lineID, w.FirstToken, w.LastToken, string(w.Kind), w.Lemma, nullString(w.LemmaReading),
			nullString(w.ConjClass), entry, sense, nullString(w.GrammarPointID), source,
		).Scan(&wordID)
		if err != nil {
			return store.Translate(err)
		}
		for step, id := range w.Steps {
			_, err := q.ExecContext(ctx,
				`INSERT INTO word_inflections (word_id, step, grammar_point_id) VALUES (?, ?, ?)`, wordID, step, id)
			if err != nil {
				return store.Translate(err)
			}
		}
	}
	return nil
}

// SaveMeanings sets the entry, sense, and conjugation class of each of words, by ID, in one transaction. Only words
// the analyzer made that still have no entry change, so a meaning you chose is never overwritten.
func (s *SQLite) SaveMeanings(ctx context.Context, words []WordAnalysis) error {
	return s.db.WithTx(ctx, func(q store.Querier) error {
		for _, w := range words {
			if w.EntryID == 0 {
				continue
			}
			_, err := q.ExecContext(ctx,
				`UPDATE line_words SET entry_id = ?, sense_index = ?, conj_class = coalesce(?, conj_class)
				 WHERE id = ? AND entry_id IS NULL AND analysis_source = 'auto'`,
				w.EntryID, w.SenseIndex, nullString(w.ConjClass), w.ID)
			if err != nil {
				return store.Translate(err)
			}
		}
		return nil
	})
}

func insertTokens(ctx context.Context, q store.Querier, lineID int64, tokens []Token) error {
	for i, t := range tokens {
		_, err := q.ExecContext(ctx,
			`INSERT INTO line_tokens (line_id, position, surface, reading, reading_source, pos)
			 VALUES (?, ?, ?, ?, 'auto', ?)`,
			lineID, i, t.Surface, nullString(t.Reading), nullString(string(t.POS)),
		)
		if err != nil {
			return store.Translate(err)
		}
	}
	return nil
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
