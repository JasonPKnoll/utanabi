package dictbuild

import (
	"reflect"
	"strings"
	"testing"
)

func TestEntryForms(t *testing.T) {
	t.Parallel()

	w := jmdictWord{
		ID: "1",
		Kanji: []jmdictKanji{
			{Text: "二", Common: true},
			{Text: "弐"},
			{Text: "弍", Tags: []string{"sK"}},
		},
		Kana: []jmdictKana{
			{Text: "に", Common: true, AppliesToKanji: []string{"*"}},
			{Text: "ふた", AppliesToKanji: []string{"二", "弐"}},
			{Text: "ニ", Tags: []string{"sk"}, AppliesToKanji: []string{}},
		},
	}
	want := []formRow{
		{form: "二", reading: "に", kanji: true, common: true},
		{form: "二", reading: "ふた", kanji: true},
		{form: "弐", reading: "に", kanji: true},
		{form: "弐", reading: "ふた", kanji: true},
		{form: "弍", reading: "に", kanji: true, tags: []string{"sK"}},
		{form: "に", reading: "に", common: true},
		{form: "ふた", reading: "ふた"},
		{form: "ニ", reading: "に", tags: []string{"sk"}},
	}
	if got := entryForms(w); !reflect.DeepEqual(got, want) {
		t.Errorf("entryForms =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEntryFormsPairsAKanjiWithNoReading(t *testing.T) {
	t.Parallel()

	w := jmdictWord{
		ID:    "1",
		Kanji: []jmdictKanji{{Text: "田んぼ", Common: true}, {Text: "田ぼ", Tags: []string{"sK"}}},
		Kana:  []jmdictKana{{Text: "たんぼ", Common: true, AppliesToKanji: []string{"田んぼ"}}},
	}
	want := []formRow{
		{form: "田んぼ", reading: "たんぼ", kanji: true, common: true},
		{form: "田ぼ", reading: "たんぼ", kanji: true, tags: []string{"sK"}},
		{form: "たんぼ", reading: "たんぼ", common: true},
	}
	if got := entryForms(w); !reflect.DeepEqual(got, want) {
		t.Errorf("entryForms =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEntryFormsNormalizesAndDropsRepeats(t *testing.T) {
	t.Parallel()

	w := jmdictWord{
		ID: "1",
		Kana: []jmdictKana{
			{Text: "ﾀﾋぬ", AppliesToKanji: []string{"*"}},
			{Text: "タヒぬ", AppliesToKanji: []string{"*"}},
		},
	}
	want := []formRow{{form: "タヒぬ", reading: "たひぬ"}}
	if got := entryForms(w); !reflect.DeepEqual(got, want) {
		t.Errorf("entryForms = %+v, want %+v", got, want)
	}
}

func TestEntrySensesKeepsJMdictOrder(t *testing.T) {
	t.Parallel()

	w := jmdictWord{Sense: []jmdictSense{
		{PartOfSpeech: []string{"v5r", "vi"}, Gloss: []jmdictGloss{{Lang: "eng", Text: "to shine"}}},
		{PartOfSpeech: []string{"v5r"}, Misc: []string{"uk"}, Info: []string{"a note"},
			Gloss: []jmdictGloss{{Lang: "ger", Text: "auffallen"}, {Lang: "eng", Text: "to stand out"}}},
	}}
	got, err := entrySenses(1, w)
	if err != nil {
		t.Fatalf("entrySenses: %v", err)
	}
	want := []senseRow{
		{glosses: []string{"to shine"}, pos: []string{"v5r", "vi"}},
		{glosses: []string{"to stand out"}, pos: []string{"v5r"}, misc: []string{"uk"}, notes: []string{"a note"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entrySenses =\n%+v\nwant\n%+v", got, want)
	}
}

func TestEntrySensesRefusesASenseWithNoEnglish(t *testing.T) {
	t.Parallel()

	w := jmdictWord{Sense: []jmdictSense{
		{Gloss: []jmdictGloss{{Lang: "eng", Text: "star"}}},
		{Gloss: []jmdictGloss{{Lang: "ger", Text: "Stern"}}},
	}}
	if _, err := entrySenses(1, w); err == nil || !strings.Contains(err.Error(), "sense 1") {
		t.Errorf("entrySenses error = %v, want one naming sense 1", err)
	}
	if _, err := entrySenses(1, jmdictWord{}); err == nil {
		t.Error("entrySenses of an entry with no senses returned no error")
	}
}

func TestDecodeJMdict(t *testing.T) {
	t.Parallel()

	const doc = `{"version":"3.6.2","languages":["eng"],"commonOnly":true,"dictDate":"2026-11-16",
		"tags":{"n":"noun (common) (futsuumeishi)"},
		"words":[{"id":"1","kanji":[],"kana":[{"common":true,"text":"ほし","tags":[],"appliesToKanji":["*"]}],
			"sense":[{"partOfSpeech":["n"],"gloss":[{"lang":"eng","text":"star"}]}]},
			{"id":"2","kanji":[],"kana":[{"text":"そら","appliesToKanji":["*"]}],
			"sense":[{"gloss":[{"lang":"eng","text":"sky"}]}]}]}`
	var ids []string
	h, err := decodeJMdict(strings.NewReader(doc), func(w jmdictWord) error {
		ids = append(ids, w.ID)
		return nil
	})
	if err != nil {
		t.Fatalf("decodeJMdict: %v", err)
	}
	if h.Version != "3.6.2" || h.DictDate != "2026-11-16" || h.Tags["n"] == "" {
		t.Errorf("header = %+v, want version 3.6.2, date 2026-11-16, and the n tag", h)
	}
	if want := []string{"1", "2"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("words = %v, want %v", ids, want)
	}
}

func TestDecodeJMdictRejectsBadFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{name: "not JSON", doc: `JMdict`, wantErr: "invalid JMdict JSON"},
		{name: "a list, not an object", doc: `[]`, wantErr: "invalid JMdict JSON"},
		{name: "cut off", doc: `{"version":"3.6.2","words":[{"id":"1"`, wantErr: "invalid JMdict JSON"},
		{name: "no words", doc: `{"version":"3.6.2","dictDate":"2026-11-16","languages":["eng"]}`, wantErr: "no words"},
		{name: "no version", doc: `{"dictDate":"2026-11-16","languages":["eng"],"words":[]}`, wantErr: "no version"},
		{
			name:    "no English",
			doc:     `{"version":"3.6.2","dictDate":"2026-11-16","languages":["ger"],"words":[]}`,
			wantErr: "no English",
		},
		{name: "words is not a list", doc: `{"words":{}}`, wantErr: "invalid JMdict JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeJMdict(strings.NewReader(tt.doc), func(jmdictWord) error { return nil })
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("decodeJMdict error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}
