package dictionary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// maxEntries caps one lookup. A kana form as short as に matches dozens of entries.
const maxEntries = 50

const maxFormLength = 100

// Dictionary is an open, read-only dictionary.db.
type Dictionary struct {
	db *sql.DB
}

// Open opens dictionary.db read-only and checks its schema version. It returns an error that wraps ErrMissing when
// there is no file at path, and one that wraps ErrVersion when the file was built for another schema.
func Open(ctx context.Context, path string) (*Dictionary, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("dictionary: no dictionary file (%s); %w", path, ErrMissing)
	} else if err != nil {
		return nil, fmt.Errorf("dictionary: check file failed (%v)", err)
	}

	// immutable tells SQLite the file never changes while it is open, so it skips locking. An app update replaces
	// the file whole, between runs. The driver is registered by internal/store.
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("immutable", "1")
	q.Add("_pragma", "query_only(1)")
	dsn := (&url.URL{Scheme: "file", Opaque: path, RawQuery: q.Encode()}).String()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("dictionary: open failed (%v)", err)
	}
	d := &Dictionary{db: db}
	if err := d.checkVersion(ctx, path); err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

func (d *Dictionary) checkVersion(ctx context.Context, path string) error {
	var version string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version)
	if err != nil {
		return fmt.Errorf("dictionary: read schema version failed (%s); %w", path, store.Translate(err))
	}
	if version != strconv.Itoa(SchemaVersion) {
		return fmt.Errorf("dictionary: file has schema version %s and the app reads %d (%s); %w",
			version, SchemaVersion, path, ErrVersion)
	}
	return nil
}

// Close closes the database.
func (d *Dictionary) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("dictionary: close failed (%v)", err)
	}
	return nil
}

// Lookup returns the entries with a spelling that matches form, and, when reading is not empty, that reading. The
// reading may be in hiragana or katakana. Entries whose matching spelling is common come first, and each comes with
// all its forms and senses, in JMdict order.
func (d *Dictionary) Lookup(ctx context.Context, form, reading string) ([]Entry, error) {
	form = jtext.Normalize(strings.TrimSpace(form))
	reading = jtext.ToHiragana(jtext.Normalize(strings.TrimSpace(reading)))
	if form == "" {
		return nil, fault.Invalid("form", "is empty")
	}
	if utf8.RuneCountInString(form) > maxFormLength || utf8.RuneCountInString(reading) > maxFormLength {
		return nil, fault.Invalid("form", fmt.Sprintf("is longer than %d characters", maxFormLength))
	}

	ids, err := d.matchingEntries(ctx, form, reading)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	idList, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("dictionary: encode entry IDs failed (%v)", err)
	}

	entries := make([]Entry, len(ids))
	byID := make(map[int64]*Entry, len(ids))
	for i, id := range ids {
		entries[i].ID = id
		byID[id] = &entries[i]
	}
	if err := d.addForms(ctx, string(idList), byID); err != nil {
		return nil, err
	}
	if err := d.addSenses(ctx, string(idList), byID); err != nil {
		return nil, err
	}
	return entries, nil
}

func (d *Dictionary) matchingEntries(ctx context.Context, form, reading string) ([]int64, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT f.entry_id
		FROM entry_forms AS f
		WHERE f.form = ?1 AND (?2 = '' OR f.reading = ?2)
		GROUP BY f.entry_id
		ORDER BY max(f.common) DESC,
			(SELECT max(a.common) FROM entry_forms AS a WHERE a.entry_id = f.entry_id) DESC,
			f.entry_id
		LIMIT ?3`, form, reading, maxEntries)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, store.Translate(err)
		}
		ids = append(ids, id)
	}
	return ids, store.Translate(rows.Err())
}

func (d *Dictionary) addForms(ctx context.Context, idList string, byID map[int64]*Entry) error {
	rows, err := d.db.QueryContext(ctx, `
		SELECT entry_id, form, reading, is_kanji, common, tags
		FROM entry_forms
		WHERE entry_id IN (SELECT value FROM json_each(?))
		ORDER BY entry_id, position`, idList)
	if err != nil {
		return store.Translate(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id   int64
			f    Form
			tags string
		)
		if err := rows.Scan(&id, &f.Text, &f.Reading, &f.Kanji, &f.Common, &tags); err != nil {
			return store.Translate(err)
		}
		if f.Tags, err = decodeList(tags); err != nil {
			return err
		}
		byID[id].Forms = append(byID[id].Forms, f)
	}
	return store.Translate(rows.Err())
}

func (d *Dictionary) addSenses(ctx context.Context, idList string, byID map[int64]*Entry) error {
	rows, err := d.db.QueryContext(ctx, `
		SELECT entry_id, sense_index, glosses, pos, misc, notes, fields, dialects, applies_to_kanji, applies_to_kana
		FROM senses
		WHERE entry_id IN (SELECT value FROM json_each(?))
		ORDER BY entry_id, sense_index`, idList)
	if err != nil {
		return store.Translate(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id    int64
			s     Sense
			lists [8]string
		)
		err := rows.Scan(&id, &s.Index, &lists[0], &lists[1], &lists[2], &lists[3], &lists[4], &lists[5], &lists[6],
			&lists[7])
		if err != nil {
			return store.Translate(err)
		}
		targets := []*[]string{&s.Glosses, &s.PartsOfSpeech, &s.Misc, &s.Notes, &s.Fields, &s.Dialects,
			&s.AppliesToKanji, &s.AppliesToKana}
		for i, target := range targets {
			if *target, err = decodeList(lists[i]); err != nil {
				return err
			}
		}
		byID[id].Senses = append(byID[id].Senses, s)
	}
	return store.Translate(rows.Err())
}

// GrammarPoint returns the grammar point with the given ID, or an error that wraps fault.ErrNotFound.
func (d *Dictionary) GrammarPoint(ctx context.Context, id string) (GrammarPoint, error) {
	if !ValidGrammarID(id) {
		return GrammarPoint{}, fault.Invalid("id", "must be 1 to 64 characters of a to z, 0 to 9, and -")
	}
	row := d.db.QueryRowContext(ctx, `
		SELECT id, kind, pattern, title, meaning, explanation, coalesce(formation, ''), coalesce(standard_id, '')
		FROM grammar_points
		WHERE id = ?`, id)
	var g GrammarPoint
	err := row.Scan(&g.ID, &g.Kind, &g.Pattern, &g.Title, &g.Meaning, &g.Explanation, &g.Formation, &g.StandardID)
	if err != nil {
		return GrammarPoint{}, store.Translate(err)
	}
	return g, nil
}

// GrammarPoints returns every grammar point: particles, then conjugations, then patterns, each sorted by ID.
func (d *Dictionary) GrammarPoints(ctx context.Context) ([]GrammarPoint, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, kind, pattern, title, meaning, explanation, coalesce(formation, ''), coalesce(standard_id, '')
		FROM grammar_points
		ORDER BY CASE kind WHEN 'particle' THEN 0 WHEN 'conjugation' THEN 1 ELSE 2 END, id`)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()

	var list []GrammarPoint
	for rows.Next() {
		var g GrammarPoint
		err := rows.Scan(&g.ID, &g.Kind, &g.Pattern, &g.Title, &g.Meaning, &g.Explanation, &g.Formation,
			&g.StandardID)
		if err != nil {
			return nil, store.Translate(err)
		}
		list = append(list, g)
	}
	return list, store.Translate(rows.Err())
}

func decodeList(s string) ([]string, error) {
	var list []string
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil, fmt.Errorf("dictionary: bad list in the database (%v)", err)
	}
	return list, nil
}
