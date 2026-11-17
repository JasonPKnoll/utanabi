//go:build integration

package store_test

import (
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

// fixture is one user song and one online song, with the line 君の声を探している fully analysed: tokens, words with
// meanings and grammar, two conjugation steps, an English translation, and one card of each kind.
const fixture = `
INSERT INTO songs (id, title, artist, source, created_at, updated_at) VALUES (1, '星の歌', 'Demo Artist', 'user', 1, 1);
INSERT INTO songs (id, title, source, source_ref, created_at, updated_at) VALUES (2, 'Online song', 'lrclib', '123', 1, 1);
INSERT INTO lyric_lines (id, song_id, position, stanza, text) VALUES
    (10, 1, 0, 0, '君の声を探している'),
    (11, 1, 1, 0, '夜の空に星が光る');
INSERT INTO line_tokens (line_id, position, surface, reading) VALUES
    (10, 0, '君', 'きみ'), (10, 1, 'の', 'の'), (10, 2, '声', 'こえ'), (10, 3, 'を', 'を'),
    (10, 4, '探し', 'さがし'), (10, 5, 'て', 'て'), (10, 6, 'いる', 'いる');
INSERT INTO line_words (id, line_id, first_token, last_token, kind, lemma, lemma_reading, conj_class, entry_id, sense_index, grammar_point_id) VALUES
    (100, 10, 0, 0, 'content', '君', 'きみ', NULL, 1, 0, NULL),
    (101, 10, 1, 1, 'particle', 'の', 'の', NULL, NULL, NULL, 'no-possession'),
    (102, 10, 2, 2, 'content', '声', 'こえ', NULL, 2, 0, NULL),
    (103, 10, 3, 3, 'particle', 'を', 'を', NULL, NULL, NULL, 'wo-object'),
    (104, 10, 4, 6, 'content', '探す', 'さがす', 'v5s', 3, 0, NULL);
INSERT INTO word_inflections (word_id, step, grammar_point_id) VALUES (104, 0, 'te-form'), (104, 1, 'teiru');
INSERT INTO line_translations (line_id, lang, text, source) VALUES (10, 'en', 'I am searching for your voice', 'user');
INSERT INTO cards (id, kind, entry_id, sense_index, context_line_id, label, label_reading, created_at)
    VALUES (1000, 'word', 3, 0, 10, '探す', 'さがす', 1);
INSERT INTO cards (id, kind, grammar_point_id, context_line_id, label, created_at)
    VALUES (1001, 'grammar', 'teiru', 10, '〜ている', 1);
INSERT INTO cards (id, kind, line_id, label, created_at) VALUES (1002, 'line', 11, '夜の空に星が光る', 1);
`

func newSchemaDB(t *testing.T) *store.DB {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Querier().ExecContext(t.Context(), fixture); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return db
}

func count(t *testing.T, db *store.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Querier().QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestSchemaSeedsTheLyricSources(t *testing.T) {
	t.Parallel()

	db := newSchemaDB(t)
	if got := count(t, db, `SELECT count(*) FROM lyric_sources WHERE code IN ('user', 'lrclib', 'netease', 'qqmusic')`); got != 4 {
		t.Errorf("lyric sources = %d, want 4", got)
	}
}

func TestSchemaAcceptsGoodRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{name: "a reading with the long vowel mark and an iteration mark",
			sql: `INSERT INTO line_tokens (line_id, position, surface, reading) VALUES (11, 0, 'ラーメン', 'らーめんゞ')`},
		{name: "a token without a reading",
			sql: `INSERT INTO line_tokens (line_id, position, surface) VALUES (11, 0, '、')`},
		{name: "a word in Latin letters without a reading",
			sql: `INSERT INTO line_tokens (line_id, position, surface) VALUES (11, 0, 'love');
			      INSERT INTO line_words (line_id, first_token, last_token, kind, lemma) VALUES (11, 0, 0, 'content', 'love')`},
		{name: "a second song from another source with the same ID",
			sql: `INSERT INTO songs (title, source, source_ref, created_at, updated_at) VALUES ('x', 'netease', '123', 1, 1)`},
		{name: "two user songs, both without a source ID",
			sql: `INSERT INTO songs (title, source, created_at, updated_at) VALUES ('x', 'user', 1, 1)`},
		{name: "a second card for a different meaning of the same word",
			sql: `INSERT INTO cards (kind, entry_id, sense_index, label, created_at) VALUES ('word', 3, 1, '探す', 1)`},
		{name: "a third conjugation step",
			sql: `INSERT INTO word_inflections (word_id, step, grammar_point_id) VALUES (104, 2, 'past')`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := newSchemaDB(t)
			if _, err := db.Querier().ExecContext(t.Context(), tt.sql, tt.args...); err != nil {
				t.Errorf("got %v, want the row accepted", err)
			}
		})
	}
}

func TestSchemaRejectsBadRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sql     string
		args    []any
		wantErr string
	}{
		{name: "song without a title", wantErr: "CHECK",
			sql: `INSERT INTO songs (title, source, created_at, updated_at) VALUES ('', 'user', 1, 1)`},
		{name: "song title over 200 characters", wantErr: "CHECK",
			sql:  `INSERT INTO songs (title, source, created_at, updated_at) VALUES (?, 'user', 1, 1)`,
			args: []any{strings.Repeat("歌", 201)}},
		{name: "song from an unknown source", wantErr: "FOREIGN KEY",
			sql: `INSERT INTO songs (title, source, source_ref, created_at, updated_at) VALUES ('x', 'spotify', '1', 1, 1)`},
		{name: "your own lyrics with a source ID", wantErr: "CHECK",
			sql: `INSERT INTO songs (title, source, source_ref, created_at, updated_at) VALUES ('x', 'user', '1', 1, 1)`},
		{name: "an online song without a source ID", wantErr: "CHECK",
			sql: `INSERT INTO songs (title, source, created_at, updated_at) VALUES ('x', 'lrclib', 1, 1)`},
		{name: "the same online song twice", wantErr: "UNIQUE",
			sql: `INSERT INTO songs (title, source, source_ref, created_at, updated_at) VALUES ('x', 'lrclib', '123', 1, 1)`},
		{name: "updated before it was created", wantErr: "CHECK",
			sql: `INSERT INTO songs (title, source, created_at, updated_at) VALUES ('x', 'user', 5, 4)`},
		{name: "line with a line break", wantErr: "CHECK",
			sql:  `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (1, 2, 0, ?)`,
			args: []any{"星が\n光る"}},
		{name: "line with a leading space", wantErr: "CHECK",
			sql: `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (1, 2, 0, ' 星')`},
		{name: "empty line", wantErr: "CHECK",
			sql: `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (1, 2, 0, '')`},
		{name: "stanza ahead of the line's position", wantErr: "CHECK",
			sql: `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (1, 2, 3, '星')`},
		{name: "two lines in the same position", wantErr: "UNIQUE",
			sql: `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (1, 1, 0, '星')`},
		{name: "line for a song that doesn't exist", wantErr: "FOREIGN KEY",
			sql: `INSERT INTO lyric_lines (song_id, position, stanza, text) VALUES (99, 0, 0, '星')`},
		{name: "reading in katakana", wantErr: "CHECK",
			sql: `INSERT INTO line_tokens (line_id, position, surface, reading) VALUES (11, 0, '夜', 'ヨル')`},
		{name: "reading with kanji in it", wantErr: "CHECK",
			sql: `INSERT INTO line_tokens (line_id, position, surface, reading) VALUES (11, 0, '夜', '夜る')`},
		{name: "word that overlaps another word", wantErr: "overlap",
			sql: `INSERT INTO line_words (line_id, first_token, last_token, kind, lemma, lemma_reading) VALUES (10, 3, 4, 'content', 'を探', 'をさが')`},
		{name: "word that ends past the last token", wantErr: "past the last token",
			sql: `INSERT INTO line_words (line_id, first_token, last_token, kind, lemma) VALUES (11, 0, 0, 'content', '夜')`},
		{name: "word moved onto another word", wantErr: "overlap",
			sql: `UPDATE line_words SET first_token = 2, last_token = 2 WHERE id = 100`},
		{name: "particle without a grammar point", wantErr: "CHECK",
			sql: `UPDATE line_words SET grammar_point_id = NULL WHERE id = 101`},
		{name: "meaning without a dictionary entry", wantErr: "CHECK",
			sql: `UPDATE line_words SET entry_id = NULL WHERE id = 100`},
		{name: "unknown kind of word", wantErr: "CHECK",
			sql: `UPDATE line_words SET kind = 'noun' WHERE id = 100`},
		{name: "badly formed conjugation class", wantErr: "CHECK",
			sql: `UPDATE line_words SET conj_class = 'Godan' WHERE id = 104`},
		{name: "badly formed grammar point ID", wantErr: "CHECK",
			sql: `UPDATE line_words SET grammar_point_id = 'Ni Place' WHERE id = 101`},
		{name: "conjugation steps with a gap", wantErr: "no gaps",
			sql: `INSERT INTO word_inflections (word_id, step, grammar_point_id) VALUES (104, 3, 'past')`},
		{name: "two conjugations in the same step", wantErr: "UNIQUE",
			sql: `INSERT INTO word_inflections (word_id, step, grammar_point_id) VALUES (104, 1, 'past')`},
		{name: "translation with a badly formed language code", wantErr: "CHECK",
			sql: `INSERT INTO line_translations (line_id, lang, text, source) VALUES (11, 'EN', 'Stars', 'user')`},
		{name: "a second translation from the same source", wantErr: "UNIQUE",
			sql: `INSERT INTO line_translations (line_id, lang, text, source) VALUES (10, 'en', 'again', 'user')`},
		{name: "word card without a meaning", wantErr: "CHECK",
			sql: `INSERT INTO cards (kind, entry_id, label, created_at) VALUES ('word', 5, 'x', 1)`},
		{name: "grammar card with a dictionary entry", wantErr: "CHECK",
			sql: `INSERT INTO cards (kind, grammar_point_id, entry_id, label, created_at) VALUES ('grammar', 'ga-subject', 5, 'x', 1)`},
		{name: "line card without a line", wantErr: "CHECK",
			sql: `INSERT INTO cards (kind, label, created_at) VALUES ('line', 'x', 1)`},
		{name: "line card with a context line", wantErr: "CHECK",
			sql: `INSERT INTO cards (kind, line_id, context_line_id, label, created_at) VALUES ('line', 10, 11, 'x', 1)`},
		{name: "a second card for the same meaning", wantErr: "UNIQUE",
			sql: `INSERT INTO cards (kind, entry_id, sense_index, label, created_at) VALUES ('word', 3, 0, '探す', 1)`},
		{name: "a second card for the same grammar point", wantErr: "UNIQUE",
			sql: `INSERT INTO cards (kind, grammar_point_id, label, created_at) VALUES ('grammar', 'teiru', 'x', 1)`},
		{name: "a second card for the same line", wantErr: "UNIQUE",
			sql: `INSERT INTO cards (kind, line_id, label, created_at) VALUES ('line', 11, 'x', 1)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := newSchemaDB(t)
			_, err := db.Querier().ExecContext(t.Context(), tt.sql, tt.args...)
			if err == nil {
				t.Fatalf("the row was accepted, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSchemaDeletingASongRemovesEverythingFromIt(t *testing.T) {
	t.Parallel()

	db := newSchemaDB(t)
	if _, err := db.Querier().ExecContext(t.Context(), `DELETE FROM songs WHERE id = 1`); err != nil {
		t.Fatalf("delete song: %v", err)
	}

	gone := map[string]string{
		"lines":           `SELECT count(*) FROM lyric_lines WHERE song_id = 1`,
		"furigana pieces": `SELECT count(*) FROM line_tokens WHERE line_id IN (10, 11)`,
		"words":           `SELECT count(*) FROM line_words WHERE line_id IN (10, 11)`,
		"inflections":     `SELECT count(*) FROM word_inflections WHERE word_id = 104`,
		"translations":    `SELECT count(*) FROM line_translations WHERE line_id IN (10, 11)`,
		"line card":       `SELECT count(*) FROM cards WHERE id = 1002`,
	}
	for what, query := range gone {
		if got := count(t, db, query); got != 0 {
			t.Errorf("%s left after deleting the song: %d", what, got)
		}
	}

	if got := count(t, db, `SELECT count(*) FROM cards WHERE id IN (1000, 1001) AND context_line_id IS NULL`); got != 2 {
		t.Errorf("word and grammar cards kept without their context line = %d, want 2", got)
	}
	if got := count(t, db, `SELECT count(*) FROM songs WHERE id = 2`); got != 1 {
		t.Errorf("the other song was deleted too")
	}
}
