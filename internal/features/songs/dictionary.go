package songs

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
)

// entryBatch is how many entries one read from dictionary.db asks for.
const entryBatch = 500

// JMdict is the Dictionary backed by internal/dictionary: JMdict meanings from dictionary.db, and the grammar list
// built into the app.
type JMdict struct {
	dict    *dictionary.Dictionary
	grammar map[string]GrammarPoint
}

// NewJMdict returns a Dictionary that reads meanings from d. A nil d means dictionary.db isn't installed: lookups
// return ErrNoMeanings, and grammar points still work.
func NewJMdict(d *dictionary.Dictionary) (*JMdict, error) {
	list, err := dictionary.Grammar()
	if err != nil {
		return nil, err
	}
	grammar := make(map[string]GrammarPoint, len(list))
	for _, g := range list {
		grammar[g.ID] = GrammarPoint{ID: g.ID, Kind: string(g.Kind), Pattern: g.Pattern, Title: g.Title,
			Meaning: g.Meaning, Explanation: g.Explanation}
	}
	return &JMdict{dict: d, grammar: grammar}, nil
}

// Lookup returns the entries for a spelling, and a reading when it isn't empty, most common first.
func (j *JMdict) Lookup(ctx context.Context, form, reading string) ([]Entry, error) {
	if j.dict == nil {
		return nil, ErrNoMeanings
	}
	found, err := j.dict.Lookup(ctx, form, reading)
	if err != nil {
		return nil, err
	}
	return entries(found), nil
}

// Entries returns the entries with the given IDs.
func (j *JMdict) Entries(ctx context.Context, ids []int64) ([]Entry, error) {
	if j.dict == nil {
		return nil, ErrNoMeanings
	}
	var list []Entry
	for start := 0; start < len(ids); start += entryBatch {
		found, err := j.dict.Entries(ctx, ids[start:min(start+entryBatch, len(ids))])
		if err != nil {
			return nil, err
		}
		list = append(list, entries(found)...)
	}
	return list, nil
}

// Tags describes dictionary codes in words.
func (j *JMdict) Tags(ctx context.Context, codes []string) (map[string]string, error) {
	if j.dict == nil {
		return nil, ErrNoMeanings
	}
	return j.dict.Tags(ctx, codes)
}

// GrammarPoint returns a grammar point from the built-in list.
func (j *JMdict) GrammarPoint(id string) (GrammarPoint, bool) {
	g, ok := j.grammar[id]
	return g, ok
}

// Attribution is JMdict's credit.
func (j *JMdict) Attribution() string {
	return dictionary.Attribution()
}

func entries(found []dictionary.Entry) []Entry {
	list := make([]Entry, len(found))
	for i, e := range found {
		list[i] = Entry{ID: e.ID, Forms: make([]Form, len(e.Forms)), Senses: make([]Sense, len(e.Senses))}
		for k, f := range e.Forms {
			list[i].Forms[k] = Form{Text: f.Text, Reading: f.Reading, Common: f.Common}
		}
		for k, s := range e.Senses {
			list[i].Senses[k] = Sense{Index: s.Index, Glosses: s.Glosses, PartsOfSpeech: s.PartsOfSpeech, Misc: s.Misc}
		}
	}
	return list
}
