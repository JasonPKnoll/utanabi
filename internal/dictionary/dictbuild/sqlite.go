package dictbuild

import (
	"cmp"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

//go:embed schema.sql
var schema string

func write(ctx context.Context, path string, jmdict io.Reader, grammar []dictionary.GrammarPoint,
	builtAt time.Time) (stats Stats, err error) {
	// The file is thrown away if the build fails, so it needs no journal. The driver is registered by internal/store.
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(OFF)")
	q.Add("_pragma", "synchronous(OFF)")
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Opaque: path, RawQuery: q.Encode()}).String())
	if err != nil {
		return Stats{}, fmt.Errorf("dictbuild: open database failed (%v)", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("dictbuild: close database failed (%v)", closeErr))
		}
	}()
	// One connection, so the pragmas above hold for every statement.
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return Stats{}, store.Translate(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Stats{}, store.Translate(err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = errors.Join(err, store.Translate(rbErr))
			}
		}
	}()

	w := &writer{tx: tx}
	header, err := decodeJMdict(jmdict, func(word jmdictWord) error { return w.addWord(ctx, word) })
	if err != nil {
		return Stats{}, err
	}
	if err := w.addTags(ctx, header.Tags); err != nil {
		return Stats{}, err
	}
	if err := addGrammar(ctx, tx, grammar); err != nil {
		return Stats{}, err
	}
	meta := map[string]string{
		"schema_version":            strconv.Itoa(dictionary.SchemaVersion),
		"jmdict_date":               header.DictDate,
		"jmdict_simplified_version": header.Version,
		"built_at":                  builtAt.UTC().Format(time.RFC3339),
	}
	for key, value := range meta {
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta (key, value) VALUES (?, ?)`, key, value); err != nil {
			return Stats{}, store.Translate(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Stats{}, store.Translate(err)
	}

	// ANALYZE gives the query planner real row counts; VACUUM packs the file, which ships with the app.
	if _, err := db.ExecContext(ctx, `ANALYZE; VACUUM`); err != nil {
		return Stats{}, store.Translate(err)
	}
	w.stats.GrammarPoints = len(grammar)
	w.stats.JMdictDate = header.DictDate
	return w.stats, nil
}

const (
	insertEntry = `INSERT INTO entries (id) VALUES (?)`
	insertForm  = `INSERT INTO entry_forms (entry_id, position, form, reading, is_kanji, common, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	insertSense = `INSERT INTO senses (entry_id, sense_index, glosses, pos, misc, notes, fields, dialects,
		applies_to_kanji, applies_to_kana) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	insertTag = `INSERT INTO tags (code, description) VALUES (?, ?)`
)

type writer struct {
	tx    *sql.Tx
	stats Stats
}

func (w *writer) addWord(ctx context.Context, word jmdictWord) error {
	id, err := entryID(word)
	if err != nil {
		return err
	}
	senses, err := entrySenses(id, word)
	if err != nil {
		return err
	}
	forms := entryForms(word)
	if len(forms) == 0 {
		return fmt.Errorf("dictbuild: JMdict entry has no spellings (%d)", id)
	}

	if _, err := w.tx.ExecContext(ctx, insertEntry, id); err != nil {
		return fmt.Errorf("dictbuild: add entry failed (%d); %w", id, store.Translate(err))
	}
	for i, f := range forms {
		tags, err := jsonList(f.tags)
		if err != nil {
			return err
		}
		if _, err := w.tx.ExecContext(ctx, insertForm, id, i, f.form, f.reading, f.kanji, f.common, tags); err != nil {
			return fmt.Errorf("dictbuild: add spelling failed (entry %d %s); %w", id, f.form, store.Translate(err))
		}
	}
	for i, s := range senses {
		lists := [][]string{s.glosses, s.pos, s.misc, s.notes, s.fields, s.dialects, s.appliesToKanji,
			s.appliesToKana}
		args := []any{id, i}
		for _, list := range lists {
			encoded, err := jsonList(list)
			if err != nil {
				return err
			}
			args = append(args, encoded)
		}
		if _, err := w.tx.ExecContext(ctx, insertSense, args...); err != nil {
			return fmt.Errorf("dictbuild: add sense failed (entry %d sense %d); %w", id, i, store.Translate(err))
		}
	}
	w.stats.Entries++
	w.stats.Forms += len(forms)
	w.stats.Senses += len(senses)
	return nil
}

func (w *writer) addTags(ctx context.Context, tags map[string]string) error {
	for code, description := range tags {
		if _, err := w.tx.ExecContext(ctx, insertTag, code, description); err != nil {
			return fmt.Errorf("dictbuild: add tag failed (%s); %w", code, store.Translate(err))
		}
	}
	w.stats.Tags = len(tags)
	return nil
}

// addGrammar inserts standard forms before the forms that point at them, so each standard_id reference is
// satisfied when its row goes in.
func addGrammar(ctx context.Context, tx *sql.Tx, grammar []dictionary.GrammarPoint) error {
	ordered := slices.Clone(grammar)
	slices.SortStableFunc(ordered, func(a, b dictionary.GrammarPoint) int {
		return cmp.Compare(len(a.StandardID), len(b.StandardID))
	})
	for _, g := range ordered {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO grammar_points (id, kind, pattern, title, meaning, explanation, formation, standard_id)
			VALUES (?, ?, ?, ?, ?, ?, nullif(?, ''), nullif(?, ''))`,
			g.ID, string(g.Kind), g.Pattern, g.Title, g.Meaning, g.Explanation, g.Formation, g.StandardID)
		if err != nil {
			return fmt.Errorf("dictbuild: add grammar point failed (%s); %w", g.ID, store.Translate(err))
		}
	}
	return nil
}

func jsonList(list []string) (string, error) {
	if list == nil {
		list = []string{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", fmt.Errorf("dictbuild: encode list failed (%v)", err)
	}
	return string(b), nil
}
