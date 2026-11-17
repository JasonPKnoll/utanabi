// Package dictionary reads dictionary.db, the read-only reference database: JMdict word meanings and Utanabi's own
// grammar point list. utanabi.db refers to it by IDs that never change: the JMdict entry number and the grammar point
// ID. The builder in dictbuild writes the file; the app only reads it.
package dictionary

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"unicode/utf8"
)

// SchemaVersion is the dictionary.db layout this code reads. The builder stores it in the meta table, and Open refuses
// a file with any other version. Bump it with every change to dictbuild/schema.sql.
const SchemaVersion = 1

var (
	// ErrMissing means there is no dictionary.db at the path. The app still runs, with word meanings unavailable.
	ErrMissing = errors.New("dictionary: file not found")
	// ErrVersion means dictionary.db was built for a different schema version. Rebuild it with task dictionary.
	ErrVersion = errors.New("dictionary: wrong schema version")
)

// Entry is one JMdict entry. ID is JMdict's own entry number (ent_seq), which never changes.
type Entry struct {
	ID     int64
	Forms  []Form
	Senses []Sense
}

// Form is one way to write an entry, paired with one reading. A kanji spelling has a row for each reading that
// applies to it (二 as に and as ふた); a kana spelling is its own reading.
type Form struct {
	Text string
	// Reading is in hiragana, even for a katakana word: テレビ reads てれび.
	Reading string
	Kanji   bool
	// Common is JMdict's priority mark: the spelling, and for a kanji spelling its reading too, is in common use.
	Common bool
	// Tags are JMdict codes for the spelling, such as rK (rarely used kanji) or sk (search-only kana).
	Tags []string
}

// Sense is one meaning of an entry. Index counts from 0 in JMdict order, and utanabi.db stores it as sense_index.
type Sense struct {
	Index   int
	Glosses []string
	// PartsOfSpeech are JMdict codes, such as v5s, v1, vs-i, vk, adj-i, adj-na, n, prt, or exp. A verb or adjective
	// code is also its conjugation class.
	PartsOfSpeech []string
	// Misc are JMdict usage codes, such as uk (usually written in kana) or col (colloquial).
	Misc []string
	// Notes are JMdict's free-text notes on the sense.
	Notes    []string
	Fields   []string
	Dialects []string
	// AppliesToKanji and AppliesToKana limit the sense to some spellings. ["*"] means every spelling.
	AppliesToKanji []string
	AppliesToKana  []string
}

// GrammarKind sorts grammar points.
type GrammarKind string

const (
	KindParticle    GrammarKind = "particle"
	KindConjugation GrammarKind = "conjugation"
	KindPattern     GrammarKind = "pattern"
)

// GrammarPoint is one use of a particle, one conjugation, or one pattern, from the hand-written list in grammar.json.
type GrammarPoint struct {
	// ID is a stable slug, such as ni-place or te-form. It is never renamed or reused.
	ID      string      `json:"id"`
	Kind    GrammarKind `json:"kind"`
	Pattern string      `json:"pattern"`
	Title   string      `json:"title"`
	// Meaning is one plain sentence for a card.
	Meaning     string `json:"meaning"`
	Explanation string `json:"explanation"`
	// Formation says how each conjugation class forms it. Every conjugation has one.
	Formation string `json:"formation,omitempty"`
	// StandardID points a spoken, short, or old form at the standard form it stands for: teru at teiru.
	StandardID string `json:"standard_id,omitempty"`
}

//go:embed grammar.json
var grammarJSON []byte

var builtInGrammar = sync.OnceValues(func() ([]GrammarPoint, error) {
	var list []GrammarPoint
	if err := json.Unmarshal(grammarJSON, &list); err != nil {
		return nil, fmt.Errorf("dictionary: read the built-in grammar list failed (%v)", err)
	}
	return list, nil
})

// Grammar returns the grammar list built into the app from grammar.json, in file order. It needs no dictionary.db,
// so grammar points can be explained even when word meanings aren't installed. dictbuild's test keeps the file valid.
func Grammar() ([]GrammarPoint, error) {
	list, err := builtInGrammar()
	if err != nil {
		return nil, err
	}
	return append([]GrammarPoint(nil), list...), nil
}

// MaxGrammarIDLength is the longest grammar point ID, the same limit utanabi.db checks.
const MaxGrammarIDLength = 64

// ValidGrammarID reports whether id has the shape utanabi.db accepts: 1 to 64 characters of a to z, 0 to 9, and -.
func ValidGrammarID(id string) bool {
	if id == "" || utf8.RuneCountInString(id) > MaxGrammarIDLength {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// Attribution is the credit JMdict's licence requires wherever the app shows dictionary data: in an About or Sources
// screen on desktop and mobile, and on each page that shows dictionary words in server mode.
func Attribution() string {
	return "Word meanings come from JMdict, the property of the Electronic Dictionary Research and Development " +
		"Group (EDRDG), used under the Creative Commons Attribution-ShareAlike 4.0 licence. " +
		"JMdict: https://www.edrdg.org/wiki/index.php/JMdict-EDICT_Dictionary_Project. " +
		"Licence: https://www.edrdg.org/edrdg/licence.html. " +
		"Converted to JSON by jmdict-simplified: https://github.com/scriptin/jmdict-simplified."
}
