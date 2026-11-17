package dictbuild

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

// The jmdict-simplified JSON format, as far as the builder reads it. The full description is at
// https://github.com/scriptin/jmdict-simplified/blob/master/docs/jmdict.md (field names are the format's own).
type jmdictHeader struct {
	Version   string
	DictDate  string
	Languages []string
	Tags      map[string]string
}

type jmdictWord struct {
	ID    string        `json:"id"`
	Kanji []jmdictKanji `json:"kanji"`
	Kana  []jmdictKana  `json:"kana"`
	Sense []jmdictSense `json:"sense"`
}

type jmdictKanji struct {
	Common bool     `json:"common"`
	Text   string   `json:"text"`
	Tags   []string `json:"tags"`
}

type jmdictKana struct {
	Common         bool     `json:"common"`
	Text           string   `json:"text"`
	Tags           []string `json:"tags"`
	AppliesToKanji []string `json:"appliesToKanji"`
}

type jmdictSense struct {
	PartOfSpeech   []string      `json:"partOfSpeech"`
	AppliesToKanji []string      `json:"appliesToKanji"`
	AppliesToKana  []string      `json:"appliesToKana"`
	Field          []string      `json:"field"`
	Dialect        []string      `json:"dialect"`
	Misc           []string      `json:"misc"`
	Info           []string      `json:"info"`
	Gloss          []jmdictGloss `json:"gloss"`
}

type jmdictGloss struct {
	Lang string `json:"lang"`
	Text string `json:"text"`
}

// decodeJMdict reads a jmdict-simplified JSON file one word at a time, so the full JMdict never has to fit in memory,
// and calls fn for each word in file order. It returns the file's header fields.
func decodeJMdict(r io.Reader, fn func(jmdictWord) error) (jmdictHeader, error) {
	var h jmdictHeader
	dec := json.NewDecoder(r)
	if err := expectDelim(dec, '{'); err != nil {
		return h, err
	}
	sawWords := false
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return h, fmt.Errorf("dictbuild: invalid JMdict JSON (%v)", err)
		}
		key, ok := tok.(string)
		if !ok {
			return h, errors.New("dictbuild: invalid JMdict JSON (an object key is not a string)")
		}

		switch key {
		case "version":
			err = decodeValue(dec, &h.Version)
		case "dictDate":
			err = decodeValue(dec, &h.DictDate)
		case "languages":
			err = decodeValue(dec, &h.Languages)
		case "tags":
			err = decodeValue(dec, &h.Tags)
		case "words":
			sawWords = true
			err = decodeWords(dec, fn)
		default:
			var skip json.RawMessage
			err = decodeValue(dec, &skip)
		}
		if err != nil {
			return h, err
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return h, err
	}

	switch {
	case !sawWords:
		return h, errors.New("dictbuild: invalid JMdict JSON (no words)")
	case h.Version == "" || h.DictDate == "":
		return h, errors.New("dictbuild: invalid JMdict JSON (no version or dictDate)")
	case !slices.Contains(h.Languages, "eng"):
		return h, fmt.Errorf("dictbuild: JMdict file has no English glosses (languages %v)", h.Languages)
	}
	return h, nil
}

func decodeWords(dec *json.Decoder, fn func(jmdictWord) error) error {
	if err := expectDelim(dec, '['); err != nil {
		return err
	}
	for dec.More() {
		var w jmdictWord
		if err := decodeValue(dec, &w); err != nil {
			return err
		}
		if err := fn(w); err != nil {
			return err
		}
	}
	return expectDelim(dec, ']')
}

type formRow struct {
	form    string
	reading string
	kanji   bool
	common  bool
	tags    []string
}

type senseRow struct {
	glosses, pos, misc, notes, fields, dialects, appliesToKanji, appliesToKana []string
}

func entryID(w jmdictWord) (int64, error) {
	id, err := strconv.ParseInt(w.ID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("dictbuild: JMdict entry ID is not a positive number (%q)", w.ID)
	}
	return id, nil
}

// entryForms pairs each kanji spelling with every reading that applies to it, then lists each kana spelling on its
// own, in JMdict order. A kanji spelling counts as common only when its reading is common too. Spellings that are the
// same after normalizing are listed once.
func entryForms(w jmdictWord) []formRow {
	var rows []formRow
	seen := make(map[[2]string]bool)
	add := func(r formRow) {
		key := [2]string{r.form, r.reading}
		if !seen[key] {
			seen[key] = true
			rows = append(rows, r)
		}
	}
	for _, k := range w.Kanji {
		paired := false
		for _, r := range w.Kana {
			if slices.Contains(r.AppliesToKanji, "*") || slices.Contains(r.AppliesToKanji, k.Text) {
				add(formRow{form: jtext.Normalize(k.Text), reading: reading(r.Text), kanji: true,
					common: k.Common && r.Common, tags: k.Tags})
				paired = true
			}
		}
		// A few search-only misspellings (tag sK) list no reading. Pair them with the entry's first reading, so a
		// lookup by that spelling still finds the entry.
		if !paired && len(w.Kana) > 0 {
			add(formRow{form: jtext.Normalize(k.Text), reading: reading(w.Kana[0].Text), kanji: true, tags: k.Tags})
		}
	}
	for _, r := range w.Kana {
		add(formRow{form: jtext.Normalize(r.Text), reading: reading(r.Text), common: r.Common, tags: r.Tags})
	}
	return rows
}

func reading(kana string) string {
	return jtext.ToHiragana(jtext.Normalize(kana))
}

func entrySenses(id int64, w jmdictWord) ([]senseRow, error) {
	if len(w.Sense) == 0 {
		return nil, fmt.Errorf("dictbuild: JMdict entry has no senses (%d)", id)
	}
	rows := make([]senseRow, 0, len(w.Sense))
	for i, s := range w.Sense {
		var glosses []string
		for _, g := range s.Gloss {
			if g.Lang == "eng" && g.Text != "" {
				glosses = append(glosses, g.Text)
			}
		}
		// Skipping the sense would shift every later sense_index away from JMdict's order, so stop instead.
		if len(glosses) == 0 {
			return nil, fmt.Errorf("dictbuild: JMdict sense has no English gloss (entry %d sense %d)", id, i)
		}
		rows = append(rows, senseRow{
			glosses: glosses, pos: s.PartOfSpeech, misc: s.Misc, notes: s.Info, fields: s.Field, dialects: s.Dialect,
			appliesToKanji: s.AppliesToKanji, appliesToKana: s.AppliesToKana,
		})
	}
	return rows, nil
}

func decodeValue(dec *json.Decoder, v any) error {
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("dictbuild: invalid JMdict JSON (%v)", err)
	}
	return nil
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("dictbuild: invalid JMdict JSON (%v)", err)
	}
	if got, ok := tok.(json.Delim); !ok || got != want {
		return fmt.Errorf("dictbuild: invalid JMdict JSON (want %v, found %v)", want, tok)
	}
	return nil
}
