package study

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
)

// entryBatch is how many entries one read from dictionary.db asks for.
const entryBatch = 500

// JMdict is the Dictionary backed by internal/dictionary.
type JMdict struct {
	dict *dictionary.Dictionary
}

// NewJMdict returns a Dictionary that reads meanings from d. A nil d means dictionary.db isn't installed, and every
// lookup returns ErrNoMeanings.
func NewJMdict(d *dictionary.Dictionary) *JMdict {
	return &JMdict{dict: d}
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
		for _, e := range found {
			entry := Entry{ID: e.ID, Forms: make([]Form, len(e.Forms)), Senses: make([]Sense, len(e.Senses))}
			for k, f := range e.Forms {
				entry.Forms[k] = Form{Text: f.Text, Reading: f.Reading}
			}
			for k, s := range e.Senses {
				entry.Senses[k] = Sense{Index: s.Index, Glosses: s.Glosses}
			}
			list = append(list, entry)
		}
	}
	return list, nil
}
