package songs

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// fakeDictionary finds entries by spelling and reading in a fixed list, and takes grammar points from the real list.
type fakeDictionary struct {
	grammar Dictionary
	entries []Entry
	tags    map[string]string
	err     error
}

func newFakeDictionary(t *testing.T) *fakeDictionary {
	t.Helper()
	sense := func(i int, gloss string, pos ...string) Sense {
		return Sense{Index: i, Glosses: []string{gloss}, PartsOfSpeech: pos}
	}
	entry := func(id int64, text, reading string, senses ...Sense) Entry {
		forms := []Form{{Text: text, Reading: reading}}
		if text != reading {
			forms = append(forms, Form{Text: reading, Reading: reading})
		}
		return Entry{ID: id, Forms: forms, Senses: senses}
	}
	uk := sense(0, "to be (of animate objects)", "v1", "vi")
	uk.Misc = []string{"uk"}
	return &fakeDictionary{
		grammar: noMeanings(t),
		entries: []Entry{
			entry(1247250, "君", "きみ", sense(0, "you", "pn"), sense(1, "monarch", "n")),
			entry(1380440, "声", "こえ", sense(0, "voice", "n")),
			entry(1593670, "探す", "さがす", sense(0, "to search for", "v5s", "vt")),
			entry(1536350, "夜", "よる", sense(0, "night", "n", "adv")),
			entry(1245280, "空", "から", sense(0, "emptiness", "n", "adj-no")),
			entry(1245290, "空", "そら", sense(0, "sky", "n")),
			entry(1376310, "星", "ほし", sense(0, "star (usu. excluding the Sun)", "n")),
			entry(1272820, "光る", "ひかる", sense(0, "to shine", "v5r", "vi")),
			entry(1198180, "会う", "あう", sense(0, "to meet", "v5u", "vi")),
			entry(1578850, "行く", "いく", sense(0, "to go", "v5k-s", "vi")),
			entry(1358280, "食べる", "たべる", sense(0, "to eat", "v1", "vt")),
			entry(1259290, "見る", "みる", sense(0, "to see", "v1", "vt")),
			entry(1547720, "来る", "くる", sense(0, "to come", "vk", "vi")),
			entry(1350040, "消える", "きえる", sense(0, "to disappear", "v1", "vi")),
			entry(1322180, "射る", "いる", sense(0, "to shoot", "v1", "vt")),
			entry(1577980, "居る", "いる", uk),
			entry(1150450, "愛する", "あいする", sense(0, "to love", "vs-s", "vt")),
			entry(1512670, "勉強", "べんきょう", sense(0, "study", "n", "vs", "vt")),
			entry(1591900, "綺麗", "きれい", sense(0, "pretty", "adj-na")),
		},
		tags: map[string]string{"v5s": "Godan verb with 'su' ending", "vt": "transitive verb", "uk": "usually kana"},
	}
}

func (f *fakeDictionary) Lookup(_ context.Context, form, reading string) ([]Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	var found []Entry
	for _, e := range f.entries {
		if slices.ContainsFunc(e.Forms, func(fm Form) bool {
			return fm.Text == form && (reading == "" || fm.Reading == reading)
		}) {
			found = append(found, e)
		}
	}
	return found, nil
}

func (f *fakeDictionary) Entries(_ context.Context, ids []int64) ([]Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	var found []Entry
	for _, e := range f.entries {
		if slices.Contains(ids, e.ID) {
			found = append(found, e)
		}
	}
	return found, nil
}

func (f *fakeDictionary) Tags(_ context.Context, codes []string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	found := map[string]string{}
	for _, c := range codes {
		if d, ok := f.tags[c]; ok {
			found[c] = d
		}
	}
	return found, nil
}

func (f *fakeDictionary) GrammarPoint(id string) (GrammarPoint, bool) {
	return f.grammar.GrammarPoint(id)
}

func (f *fakeDictionary) Attribution() string { return "JMdict credit" }

func analyzedLine(t *testing.T, dict Dictionary, text string) Line {
	t.Helper()
	line := Line{Text: text, Tokens: analyzeText(t, morph.New(), text)}
	if err := newMeanings(dict).analyzeWords(t.Context(), &line); err != nil {
		t.Fatalf("analyzeWords(%q): %v", text, err)
	}
	return line
}

func TestWordsTakeTheirMeaningFromTheDictionary(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text string
		want []string
	}{
		{text: "君の声を探している", want: []string{
			"君 1247250/0 ", "の 0/0 ", "声 1380440/0 ", "を 0/0 ", "探す 1593670/0 v5s",
		}},
		{text: "夜の空に星が光る", want: []string{
			"夜 1536350/0 ", "の 0/0 ", "空 1245290/0 ", "に 0/0 ", "星 1376310/0 ", "が 0/0 ", "光る 1272820/0 v5r",
		}},
		{text: "家にいる", want: []string{"家 0/0 ", "に 0/0 ", "いる 1577980/0 v1"}},
		{text: "愛する", want: []string{"愛する 1150450/0 vs-s"}},
		{text: "勉強する", want: []string{"勉強する 1512670/0 vs"}},
		{text: "綺麗な", want: []string{"綺麗 1591900/0 adj-na"}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := analyzedLine(t, dict, tt.text)
			got := make([]string, len(line.Analysis))
			for i, w := range line.Analysis {
				got[i] = w.Lemma + " " + strconv.FormatInt(w.EntryID, 10) + "/" + strconv.Itoa(w.SenseIndex) + " " + w.ConjClass
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestPotentialVerbsBecomeTheirGodanVerb(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text string
		want []string
	}{
		{text: "会えない", want: []string{"会えない 会う/あう content v5u [potential negative]"}},
		{text: "見れる", want: []string{"見れる 見る/みる content v1 [ra-nuki]"}},
		{text: "来れる", want: []string{"来れる 来る/くる content vk [ra-nuki]"}},
		{text: "消えない", want: []string{"消えない 消える/きえる content v1 [negative]"}},
		{text: "食べさせられなかった", want: []string{
			"食べさせられなかった 食べる/たべる content v1 [causative passive negative past]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := analyzedLine(t, dict, tt.text)
			if got := describeWords(line.Tokens, line.Analysis); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestPotentialVerbsStayAsReadWithoutTheDictionary(t *testing.T) {
	t.Parallel()

	line := analyzedLine(t, noMeanings(t), "会えない")
	want := []string{"会えない 会える/あえる content v1 [negative]"}
	if got := describeWords(line.Tokens, line.Analysis); !reflect.DeepEqual(got, want) {
		t.Errorf("words = %q, want %q", got, want)
	}
	if line.Analysis[0].EntryID != 0 {
		t.Errorf("entry = %d, want none without the dictionary", line.Analysis[0].EntryID)
	}
}

func TestPickPrefersAFittingSenseThenTheClassThenKana(t *testing.T) {
	t.Parallel()

	noun := Entry{ID: 1, Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}}}}
	laterVerb := Entry{ID: 2, Senses: []Sense{
		{Index: 0, PartsOfSpeech: []string{"n"}}, {Index: 1, PartsOfSpeech: []string{"v5r", "vi"}},
	}}
	godan := Entry{ID: 3, Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"v5r"}}}}
	ichidan := Entry{ID: 4, Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"v1"}}}}
	kana := Entry{ID: 5, Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"v1"}, Misc: []string{"uk"}}}}
	// だれ: 垂れ "sauce" is rare but usually written in kana; 誰 "who" is common.
	sauce := Entry{ID: 6, Forms: []Form{{Text: "たれ", Reading: "たれ", Common: true}, {Text: "だれ", Reading: "だれ"}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}, Misc: []string{"uk"}}}}
	who := Entry{ID: 7, Forms: []Form{{Text: "誰", Reading: "だれ", Common: true}, {Text: "だれ", Reading: "だれ", Common: true}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"pn"}}}}
	commonNoun := Entry{ID: 8, Forms: []Form{{Text: "た", Reading: "た", Common: true}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}}}}
	tests := []struct {
		name      string
		entries   []Entry
		cat       category
		class     string
		form      string
		wantID    int64
		wantSense int
	}{
		{name: "the first sense that fits", entries: []Entry{noun, laterVerb}, cat: catVerb, wantID: 2, wantSense: 1},
		{name: "nothing fits takes the first", entries: []Entry{noun}, cat: catVerb, wantID: 1, wantSense: 0},
		{name: "the word's class", entries: []Entry{godan, ichidan}, cat: catVerb, class: "v1", wantID: 4},
		{name: "kana for a kana word", entries: []Entry{ichidan, kana}, cat: catVerb, class: "v1", form: "いる",
			wantID: 5},
		{name: "dictionary order on a tie", entries: []Entry{ichidan, kana}, cat: catVerb, class: "v1", form: "居る",
			wantID: 4},
		{name: "the word's common spelling over a rare kana one", entries: []Entry{sauce, who}, cat: catNoun,
			form: "だれ", wantID: 7},
		{name: "common in katakana too", entries: []Entry{sauce, who}, cat: catNoun, form: "ダレ", wantID: 7},
		{name: "a fitting sense over a common word", entries: []Entry{commonNoun, ichidan}, cat: catVerb, wantID: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e, sense, ok := pick(tt.entries, tt.cat, tt.class, tt.form)
			if !ok || e.ID != tt.wantID || sense != tt.wantSense {
				t.Errorf("pick = %d sense %d (%v), want %d sense %d", e.ID, sense, ok, tt.wantID, tt.wantSense)
			}
		})
	}
	if _, _, ok := pick(nil, catNoun, "", ""); ok {
		t.Error("pick(nil) found an entry")
	}
}

func TestWordsFailWhenTheDictionaryFails(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	dict.err = errors.New("dictionary: query failed")
	line := Line{Text: "星", Tokens: analyzeText(t, morph.New(), "星")}
	if err := newMeanings(dict).analyzeWords(t.Context(), &line); !errors.Is(err, dict.err) {
		t.Errorf("err = %v, want the dictionary's error", err)
	}
}

func TestLookupWordRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lemma     string
		reading   string
		entryID   int64
		wantField string
	}{
		{name: "no lemma", lemma: " ", wantField: "lemma"},
		{name: "lemma too long", lemma: string(make([]rune, 101)), wantField: "lemma"},
		{name: "reading too long", lemma: "星", reading: string(make([]rune, 101)), wantField: "reading"},
		{name: "reading not kana", lemma: "星", reading: "hoshi", wantField: "reading"},
		{name: "negative entry", lemma: "星", entryID: -1, wantField: "entryId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := LookupWord(t.Context(), newFakeDictionary(t), tt.lemma, tt.reading, tt.entryID)
			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != tt.wantField {
				t.Errorf("err = %v, want invalid input on %s", err, tt.wantField)
			}
		})
	}
}

func TestLookupWordWithoutTheDictionary(t *testing.T) {
	t.Parallel()

	got, err := LookupWord(t.Context(), noMeanings(t), "星", "ほし", 0)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	if got.Installed || got.Senses == nil || len(got.Senses) != 0 {
		t.Errorf("LookupWord = %+v, want not installed and no senses", got)
	}
}

func TestLookupWordDescribesEverySense(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	got, err := LookupWord(t.Context(), dict, "探す", "サガス", 0)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	want := Meanings{Installed: true, EntryID: 1593670, Attribution: "JMdict credit", Senses: []Meaning{{
		Index: 0, Glosses: []string{"to search for"}, PartsOfSpeech: []string{"Godan verb with 'su' ending", "transitive verb"},
		Notes: []string{},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LookupWord = %+v, want %+v", got, want)
	}
}

func TestLookupWordPrefersTheWordsOwnEntry(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	got, err := LookupWord(t.Context(), dict, "空", "", 1245290)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	if got.EntryID != 1245290 || got.Senses[0].Glosses[0] != "sky" {
		t.Errorf("LookupWord(空, 1245290) = entry %d %q, want 1245290 sky", got.EntryID, got.Senses[0].Glosses)
	}

	first, err := LookupWord(t.Context(), dict, "空", "", 0)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	if first.EntryID != 1245280 {
		t.Errorf("LookupWord(空) = entry %d, want the first, 1245280", first.EntryID)
	}

	none, err := LookupWord(t.Context(), dict, "未知語", "", 0)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	if !none.Installed || none.EntryID != 0 || len(none.Senses) != 0 || none.Attribution == "" {
		t.Errorf("LookupWord(未知語) = %+v, want installed with no senses and the credit", none)
	}
}
