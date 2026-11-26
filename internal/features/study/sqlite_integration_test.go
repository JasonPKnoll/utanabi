//go:build integration

package study

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

// A song of two lines: 星が光った (光る, entry 1) and 君がいる 光る (君, entry 2, then 光る again), with particles and a
// word that has no dictionary meaning, neither of which is a card.
const fixture = `
INSERT INTO songs (id, title, source, created_at, updated_at) VALUES (1, '星の歌', 'user', 1, 1);
INSERT INTO library_songs (owner, song_id, added_at) VALUES (0, 1, 1);
INSERT INTO lyric_lines (id, song_id, position, stanza, text) VALUES (10, 1, 0, 0, '星が光った'), (11, 1, 1, 0, '君がいる 光る');
INSERT INTO line_tokens (line_id, position, surface, reading) VALUES
    (10, 0, '星', 'ほし'), (10, 1, 'が', 'が'), (10, 2, '光っ', 'ひかっ'), (10, 3, 'た', 'た'),
    (11, 0, '君', 'きみ'), (11, 1, 'が', 'が'), (11, 2, 'いる', 'いる'), (11, 3, ' ', NULL), (11, 4, '光る', 'ひかる');
INSERT INTO line_words (line_id, first_token, last_token, kind, lemma, lemma_reading, entry_id, sense_index, grammar_point_id) VALUES
    (10, 0, 0, 'content', '星', 'ほし', NULL, NULL, NULL),
    (10, 1, 1, 'particle', 'が', 'が', NULL, NULL, 'ga-subject'),
    (10, 2, 3, 'content', '光る', 'ひかる', 1, 0, NULL),
    (11, 0, 0, 'content', '君', 'きみ', 2, 0, NULL),
    (11, 1, 1, 'particle', 'が', 'が', NULL, NULL, 'ga-subject'),
    (11, 4, 4, 'content', '光る', 'ひかる', 1, 0, NULL);
`

func newRepo(t *testing.T) *SQLite {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Querier().ExecContext(t.Context(), fixture); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return NewSQLite(db, identity.OnDevice())
}

func TestSongWordsAreTheMeaningfulWordsInOrder(t *testing.T) {
	t.Parallel()

	title, got, err := newRepo(t).SongWords(t.Context(), 1)
	if err != nil {
		t.Fatalf("SongWords: %v", err)
	}
	if title != "星の歌" {
		t.Errorf("title = %q, want 星の歌", title)
	}
	want := []SongWord{
		{EntryID: 1, Lemma: "光る", LemmaReading: "ひかる", LineID: 10, LineText: "星が光った", WordStart: 2, WordText: "光った"},
		{EntryID: 2, Lemma: "君", LemmaReading: "きみ", LineID: 11, LineText: "君がいる 光る", WordStart: 0, WordText: "君"},
		{EntryID: 1, Lemma: "光る", LemmaReading: "ひかる", LineID: 11, LineText: "君がいる 光る", WordStart: 5, WordText: "光る"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SongWords\n got  %+v\n want %+v", got, want)
	}
}

func TestSongWordsOfAMissingSong(t *testing.T) {
	t.Parallel()

	if _, _, err := newRepo(t).SongWords(t.Context(), 99); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("SongWords(99) error = %v, want not found", err)
	}
}

func TestWordCardsAreSavedOnceAndKeepTheirProgress(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	_, words, err := repo.SongWords(t.Context(), 1)
	if err != nil {
		t.Fatalf("SongWords: %v", err)
	}
	now := time.UnixMilli(1000)
	first, err := repo.WordCards(t.Context(), words[:2], now)
	if err != nil {
		t.Fatalf("WordCards: %v", err)
	}
	if len(first) != 2 || first[0].Label != "光る" || first[0].LabelReading != "ひかる" || first[1].Label != "君" {
		t.Fatalf("WordCards = %+v, want 光る and 君", first)
	}
	if _, err := repo.RecordAnswer(t.Context(), first[0].ID, true, now); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}

	again, err := repo.WordCards(t.Context(), words, now)
	if err != nil {
		t.Fatalf("WordCards again: %v", err)
	}
	if again[0].ID != first[0].ID || again[2].ID != first[0].ID || again[1].ID != first[1].ID {
		t.Errorf("cards the second time = %+v, want the same cards, 光る both times", again)
	}
	if again[0].Progress != (Progress{Streak: 1, Right: 1}) {
		t.Errorf("光る progress = %+v, want one right answer", again[0].Progress)
	}
}

func TestRecordAnswerCountsTheStreak(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	_, words, err := repo.SongWords(t.Context(), 1)
	if err != nil {
		t.Fatalf("SongWords: %v", err)
	}
	cards, err := repo.WordCards(t.Context(), words[:1], time.UnixMilli(1))
	if err != nil {
		t.Fatalf("WordCards: %v", err)
	}
	id := cards[0].ID
	answers := []struct {
		right bool
		want  Progress
	}{
		{true, Progress{Streak: 1, Right: 1}},
		{true, Progress{Streak: 2, Right: 2}},
		{false, Progress{Streak: 0, Right: 2, Wrong: 1}},
		{true, Progress{Streak: 1, Right: 3, Wrong: 1}},
	}
	for i, a := range answers {
		got, err := repo.RecordAnswer(t.Context(), id, a.right, time.UnixMilli(int64(10+i)))
		if err != nil {
			t.Fatalf("RecordAnswer %d: %v", i, err)
		}
		if got != a.want {
			t.Errorf("after answer %d (%v): %+v, want %+v", i, a.right, got, a.want)
		}
	}
	if _, err := repo.RecordAnswer(t.Context(), 9999, true, time.UnixMilli(1)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("RecordAnswer(missing card) error = %v, want not found", err)
	}
}

func TestCardsAndProgressAreEachUsersOwn(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Querier().ExecContext(t.Context(), fixture+`
		INSERT INTO users (id, handle, name, created_at) VALUES (1, randomblob(32), 'hana', 1), (2, randomblob(32), 'kaze', 1);
		INSERT INTO library_songs (owner, song_id, added_at) VALUES (1, 1, 1), (2, 1, 1);`); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	repo := NewSQLite(db, identity.Accounts())
	hana := identity.Into(t.Context(), identity.User{ID: 1, Name: "hana"})
	kaze := identity.Into(t.Context(), identity.User{ID: 2, Name: "kaze"})

	_, words, err := repo.SongWords(hana, 1)
	if err != nil {
		t.Fatalf("SongWords: %v", err)
	}
	hanas, err := repo.WordCards(hana, words[:1], time.UnixMilli(1))
	if err != nil {
		t.Fatalf("WordCards hana: %v", err)
	}
	if _, err := repo.RecordAnswer(hana, hanas[0].ID, true, time.UnixMilli(2)); err != nil {
		t.Fatalf("RecordAnswer: %v", err)
	}
	kazes, err := repo.WordCards(kaze, words[:1], time.UnixMilli(1))
	if err != nil {
		t.Fatalf("WordCards kaze: %v", err)
	}
	if kazes[0].ID == hanas[0].ID || kazes[0].Progress != (Progress{}) {
		t.Errorf("kaze's card = %+v, want a card of his own with no progress", kazes[0])
	}
	if _, err := repo.RecordAnswer(kaze, hanas[0].ID, false, time.UnixMilli(3)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("kaze answering hana's card: %v, want not found", err)
	}
	if _, _, err := repo.SongWords(t.Context(), 1); !errors.Is(err, fault.ErrUnauthorized) {
		t.Errorf("SongWords with no one signed in: %v, want unauthorized", err)
	}
}

func TestSongWordsOfASongInSomeoneElsesLibrary(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Querier().ExecContext(t.Context(), fixture+`
		INSERT INTO users (id, handle, name, created_at) VALUES (1, randomblob(32), 'hana', 1);`); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	hana := identity.Into(t.Context(), identity.User{ID: 1, Name: "hana"})
	if _, _, err := NewSQLite(db, identity.Accounts()).SongWords(hana, 1); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("hana reading the device's song: %v, want not found", err)
	}
}
