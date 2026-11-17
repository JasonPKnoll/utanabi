//go:build integration

package dictionary_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
)

// buildFixture builds a small dictionary.db from testdata through the real builder, with the real grammar list.
func buildFixture(t *testing.T) string {
	t.Helper()
	jmdict, err := os.Open(filepath.Join("testdata", "jmdict-eng-common-fixture.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer jmdict.Close()
	grammarFile, err := os.Open("grammar.json")
	if err != nil {
		t.Fatalf("open grammar list: %v", err)
	}
	defer grammarFile.Close()
	grammar, err := dictbuild.ReadGrammar(grammarFile)
	if err != nil {
		t.Fatalf("ReadGrammar: %v", err)
	}

	path := filepath.Join(t.TempDir(), "dictionary.db")
	if _, err := dictbuild.Build(t.Context(), path, jmdict, grammar, time.Now()); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return path
}

func openFixture(t *testing.T) *dictionary.Dictionary {
	t.Helper()
	d, err := dictionary.Open(t.Context(), buildFixture(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return d
}

func ids(entries []dictionary.Entry) []int64 {
	list := make([]int64, len(entries))
	for i, e := range entries {
		list[i] = e.ID
	}
	return list
}

func TestLookupReturnsFormsAndSensesInOrder(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	entries, err := d.Lookup(t.Context(), "光る", "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Lookup(光る) found %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != 1272820 {
		t.Errorf("ID = %d, want JMdict's 1272820", e.ID)
	}
	wantForms := []dictionary.Form{
		{Text: "光る", Reading: "ひかる", Kanji: true, Common: true, Tags: []string{}},
		{Text: "ひかる", Reading: "ひかる", Common: true, Tags: []string{}},
	}
	if !reflect.DeepEqual(e.Forms, wantForms) {
		t.Errorf("Forms = %+v, want %+v", e.Forms, wantForms)
	}
	if len(e.Senses) != 1 {
		t.Fatalf("光る has %d senses, want 1", len(e.Senses))
	}
	first := e.Senses[0]
	if want := []string{"to shine", "to glitter", "to be bright"}; !slices.Equal(first.Glosses, want) {
		t.Errorf("sense 0 glosses = %q, want %q", first.Glosses, want)
	}
	if want := []string{"v5r", "vi"}; !slices.Equal(first.PartsOfSpeech, want) {
		t.Errorf("sense 0 parts of speech = %q, want %q", first.PartsOfSpeech, want)
	}
	if !slices.Equal(first.AppliesToKanji, []string{"*"}) {
		t.Errorf("sense 0 applies to kanji %q, want [*]", first.AppliesToKanji)
	}
}

func TestLookupKeepsSenseOrder(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	entries, err := d.Lookup(t.Context(), "君", "きみ")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Lookup(君, きみ) found %d entries, want 1", len(entries))
	}
	senses := entries[0].Senses
	if len(senses) < 3 {
		t.Fatalf("君 has %d senses, want at least 3", len(senses))
	}
	for i, s := range senses {
		if s.Index != i {
			t.Errorf("sense %d has Index %d, want senses in JMdict order from 0", i, s.Index)
		}
	}
	wantFirstGloss := []string{"you", "monarch", "he"}
	for i, want := range wantFirstGloss {
		if got := senses[i].Glosses[0]; got != want {
			t.Errorf("sense %d first gloss = %q, want %q", i, got, want)
		}
	}
	if !slices.Equal(senses[0].Misc, []string{"fam"}) || len(senses[0].Notes) == 0 {
		t.Errorf("sense 0 misc = %q and notes = %q, want [fam] and JMdict's note", senses[0].Misc, senses[0].Notes)
	}
	if !slices.Equal(senses[2].Misc, []string{"pol", "obs"}) {
		t.Errorf("sense 2 misc = %q, want [pol obs]", senses[2].Misc)
	}
}

func TestLookupPutsCommonSpellingsFirst(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	// が is the common particle (2028930), and a rare reading of 画 in the entry for 絵 (1202270), whose ID is
	// lower. The common one comes first.
	entries, err := d.Lookup(t.Context(), "が", "")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if want := []int64{2028930, 1202270}; !slices.Equal(ids(entries), want) {
		t.Errorf("Lookup(が) = %v, want %v", ids(entries), want)
	}
	if !slices.Contains(entries[0].Senses[0].PartsOfSpeech, "prt") {
		t.Errorf("first が sense parts of speech = %q, want prt", entries[0].Senses[0].PartsOfSpeech)
	}
}

func TestLookupFiltersByReading(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	tests := []struct {
		name    string
		form    string
		reading string
		want    []int64
	}{
		{name: "both readings of 空", form: "空", want: []int64{1245280, 1245290}},
		{name: "空 read そら", form: "空", reading: "そら", want: []int64{1245290}},
		{name: "空 read から", form: "空", reading: "から", want: []int64{1245280}},
		{name: "a katakana reading matches", form: "空", reading: "ソラ", want: []int64{1245290}},
		{name: "君 read くん", form: "君", reading: "くん", want: []int64{1247260}},
		{name: "a reading of another kanji", form: "空", reading: "ほし", want: nil},
		{name: "kana spelling", form: "さがす", want: []int64{1593670}},
		{name: "second kanji spelling", form: "捜す", reading: "さがす", want: []int64{1593670}},
		{name: "katakana spelling of a kana word", form: "カラ", want: []int64{1245280}},
		{name: "spaces around the form", form: " 星 ", want: []int64{1376310}},
		{name: "not in the dictionary", form: "未知語", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entries, err := d.Lookup(t.Context(), tt.form, tt.reading)
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			if got := ids(entries); !slices.Equal(got, tt.want) {
				t.Errorf("Lookup(%q, %q) = %v, want %v", tt.form, tt.reading, got, tt.want)
			}
		})
	}
}

func TestLookupPairsEveryReadingOfAKanjiSpelling(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	entries, err := d.Lookup(t.Context(), "二", "ふた")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != 1461140 {
		t.Fatalf("Lookup(二, ふた) = %v, want [1461140]", ids(entries))
	}
	var readings []string
	for _, f := range entries[0].Forms {
		if f.Text == "二" {
			readings = append(readings, f.Reading)
		}
	}
	if want := []string{"に", "ふた", "ふ", "ふう"}; !slices.Equal(readings, want) {
		t.Errorf("readings of 二 = %q, want %q", readings, want)
	}
}

func TestLookupRejectsBadInput(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	tests := []struct {
		name    string
		form    string
		reading string
	}{
		{name: "empty form", form: ""},
		{name: "blank form", form: "  "},
		{name: "form too long", form: string(make([]rune, 101))},
		{name: "reading too long", form: "空", reading: string(make([]rune, 101))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := d.Lookup(t.Context(), tt.form, tt.reading)
			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != "form" {
				t.Errorf("Lookup error = %v, want invalid input on form", err)
			}
		})
	}
}

func TestGrammarPoint(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	got, err := d.GrammarPoint(t.Context(), "teru")
	if err != nil {
		t.Fatalf("GrammarPoint: %v", err)
	}
	if got.Kind != dictionary.KindConjugation || got.Pattern != "〜てる" || got.StandardID != "teiru" ||
		got.Formation == "" || got.Meaning == "" {
		t.Errorf("GrammarPoint(teru) = %+v, want a conjugation 〜てる with a formation and standard teiru", got)
	}

	particle, err := d.GrammarPoint(t.Context(), "ni-place")
	if err != nil {
		t.Fatalf("GrammarPoint: %v", err)
	}
	if particle.Kind != dictionary.KindParticle || particle.Formation != "" || particle.StandardID != "" {
		t.Errorf("GrammarPoint(ni-place) = %+v, want a particle with no formation or standard", particle)
	}

	if _, err := d.GrammarPoint(t.Context(), "no-such-point"); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("GrammarPoint(no-such-point) error = %v, want fault.ErrNotFound", err)
	}
	var invalid *fault.InvalidError
	if _, err := d.GrammarPoint(t.Context(), "Not A Slug"); !errors.As(err, &invalid) {
		t.Errorf("GrammarPoint(Not A Slug) error = %v, want invalid input", err)
	}
}

func TestGrammarPointsMatchesTheList(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	list, err := d.GrammarPoints(t.Context())
	if err != nil {
		t.Fatalf("GrammarPoints: %v", err)
	}

	f, err := os.Open("grammar.json")
	if err != nil {
		t.Fatalf("open grammar list: %v", err)
	}
	defer f.Close()
	want, err := dictbuild.ReadGrammar(f)
	if err != nil {
		t.Fatalf("ReadGrammar: %v", err)
	}
	slices.SortStableFunc(want, func(a, b dictionary.GrammarPoint) int {
		order := map[dictionary.GrammarKind]int{"particle": 0, "conjugation": 1, "pattern": 2}
		if d := order[a.Kind] - order[b.Kind]; d != 0 {
			return d
		}
		if a.ID < b.ID {
			return -1
		}
		return 1
	})
	if !reflect.DeepEqual(list, want) {
		t.Errorf("GrammarPoints differs from grammar.json sorted by kind and ID")
	}
}

func TestEntriesReturnsEntriesByID(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	entries, err := d.Entries(t.Context(), []int64{1593670, 999999999, 1272820, 1593670})
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if want := []int64{1593670, 1272820}; !slices.Equal(ids(entries), want) {
		t.Fatalf("Entries = %v, want %v, in the order asked and without the unknown ID", ids(entries), want)
	}
	if got := entries[0].Senses[0].Glosses[0]; got != "to search for" {
		t.Errorf("探す first gloss = %q, want %q", got, "to search for")
	}
	if len(entries[1].Forms) != 2 {
		t.Errorf("光る has %d forms, want 2", len(entries[1].Forms))
	}

	none, err := d.Entries(t.Context(), nil)
	if err != nil || len(none) != 0 {
		t.Errorf("Entries(nil) = %v, %v, want nothing and no error", none, err)
	}
}

func TestEntriesRejectsTooManyIDs(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	_, err := d.Entries(t.Context(), make([]int64, 2001))
	var invalid *fault.InvalidError
	if !errors.As(err, &invalid) || invalid.Field != "ids" {
		t.Errorf("Entries error = %v, want invalid input on ids", err)
	}
}

func TestTagsDescribesKnownCodes(t *testing.T) {
	t.Parallel()

	d := openFixture(t)
	tags, err := d.Tags(t.Context(), []string{"pref", "suf", "no-such-code"})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	want := map[string]string{"pref": "prefix", "suf": "suffix"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("Tags = %v, want %v", tags, want)
	}

	var invalid *fault.InvalidError
	if _, err := d.Tags(t.Context(), make([]string, 501)); !errors.As(err, &invalid) {
		t.Errorf("Tags with 501 codes error = %v, want invalid input", err)
	}
}

func TestOpenMissingFile(t *testing.T) {
	t.Parallel()

	_, err := dictionary.Open(t.Context(), filepath.Join(t.TempDir(), "dictionary.db"))
	if !errors.Is(err, dictionary.ErrMissing) {
		t.Errorf("Open error = %v, want dictionary.ErrMissing", err)
	}
}

func TestOpenRefusesAnotherSchemaVersion(t *testing.T) {
	t.Parallel()

	path := buildFixture(t)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE meta SET value = '999' WHERE key = 'schema_version'`); err != nil {
		t.Fatalf("change schema version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err = dictionary.Open(t.Context(), path)
	if !errors.Is(err, dictionary.ErrVersion) {
		t.Errorf("Open error = %v, want dictionary.ErrVersion", err)
	}
}

func TestOpenRefusesAFileThatIsNotADictionary(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "dictionary.db")
	if err := os.WriteFile(path, []byte("not a database"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, err := dictionary.Open(t.Context(), path)
	if err == nil || errors.Is(err, dictionary.ErrMissing) || errors.Is(err, dictionary.ErrVersion) {
		t.Errorf("Open error = %v, want a plain failure", err)
	}
}

func TestOpenIsReadOnly(t *testing.T) {
	t.Parallel()

	path := buildFixture(t)
	d, err := dictionary.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path + "-wal"); !os.IsNotExist(err) {
		t.Errorf("Stat(-wal) = %v, want no write-ahead log next to a read-only dictionary", err)
	}
}
