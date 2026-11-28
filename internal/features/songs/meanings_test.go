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
			entry(1221520, "気", "き", sense(0, "spirit", "n")),
			entry(1495740, "付く", "つく", sense(0, "to be attached", "v5k", "vi")),
			{ID: 1591050, Forms: []Form{{Text: "気がつく", Reading: "きがつく"}, {Text: "気が付く", Reading: "きがつく"}},
				Senses: []Sense{sense(0, "to notice", "exp", "v5k")}},
			entry(1395570, "難", "なん", sense(0, "difficulty", "n")),
			entry(1188490, "何", "なん", sense(0, "what", "pn")),
			entry(1165970, "一番", "いちばん", sense(0, "number one", "n", "adv")),
			entry(1482660, "二人", "ふたり", sense(0, "two people", "n")),
			{ID: 1188500, Forms: []Form{{Text: "何も", Reading: "なにも"}, {Text: "なにも", Reading: "なにも"},
				{Text: "なんも", Reading: "なんも"}}, Senses: []Sense{sense(0, "(not) anything", "exp")}},
			entry(1529520, "無い", "ない", sense(0, "nonexistent", "adj-i")),
			entry(1532800, "迷惑", "めいわく", sense(0, "trouble", "n", "adj-na")),
			entry(1207610, "掛ける", "かける", sense(0, "to hang up", "v1", "vt")),
			{ID: 1911730, Forms: []Form{{Text: "迷惑をかける", Reading: "めいわくをかける"}},
				Senses: []Sense{sense(0, "to cause trouble", "exp", "v1")}},
			entry(2100160, "ググる", "ぐぐる", sense(0, "to google", "v5r", "vt")),
			entry(2746990, "はんぱない", "はんぱない", sense(0, "to a great extent", "adj-i")),
			entry(1599780, "臭う", "におう", sense(0, "to be fragrant", "v5u", "vi")),
			entry(1333150, "臭い", "くさい", sense(0, "stinking", "adj-i")),
			{ID: 1268570, Forms: []Form{{Text: "五日", Reading: "いつか"}, {Text: "いつか", Reading: "いつか", Common: true}},
				Senses: []Sense{sense(0, "5th day of the month", "n")}},
			{ID: 1188790, Forms: []Form{{Text: "何時か", Reading: "いつか"}, {Text: "いつか", Reading: "いつか"}},
				Senses: []Sense{{Index: 0, Glosses: []string{"sometime"}, PartsOfSpeech: []string{"adv"}, Misc: []string{"uk"}}}},
			entry(1595730, "全て", "すべて", sense(0, "everything", "n"), sense(1, "entirely", "adv")),
			entry(1058210, "サン", "さん", sense(0, "Sun", "n")),
			entry(2747950, "サン", "さん", sense(0, "three", "num")),
			entry(1579470, "ヨン", "よん", sense(0, "four", "num")),
			{ID: 2839962, Forms: []Form{{Text: "ゼロ", Reading: "ぜろ"}},
				Senses: []Sense{{Index: 0, Glosses: []string{"zero", "0"}, PartsOfSpeech: []string{"n"}}}},
			entry(1149130, "ワン", "わん", sense(0, "one", "num")),
			entry(1217700, "頑張る", "がんばる", sense(0, "to persevere", "v5r", "vi")),
			entry(1386560, "仕方ない", "しかたない", sense(0, "it can't be helped", "adj-i")),
			entry(1601440, "分け隔て", "わけへだて", sense(0, "discrimination", "n", "vs")),
			entry(1499960, "分ける", "わける", sense(0, "to divide", "v1", "vt")),
			entry(1489590, "隔て", "へだて", sense(0, "partition", "n")),
			entry(1527980, "分け", "わけ", sense(0, "division", "n")),
			entry(1600820, "訳", "わけ", sense(0, "conclusion from reasoning", "n")),
			entry(1525360, "まとめて", "まとめて", sense(0, "all together", "exp", "adv")),
			entry(1525350, "纏める", "まとめる", sense(0, "to collect", "v1", "vt")),
			entry(1207590, "掛かる", "かかる", sense(0, "to take (time)", "v5r", "vi")),
			entry(2084130, "かかってこい", "かかってこい", sense(0, "bring it on!", "int")),
			entry(1323170, "知る", "しる", sense(0, "to know", "v5r", "vt")),
			entry(2137720, "知ったこっちゃない", "しったこっちゃない", sense(0, "I don't give a damn", "exp", "adj-i")),
			entry(1313580, "こと", "こと", sense(0, "thing", "n")),
			entry(1303980, "肩", "かた", sense(0, "shoulder", "n")),
			entry(1374630, "落とす", "おとす", sense(0, "to drop", "v5s", "vt")),
			entry(1578860, "行ける", "いける", sense(0, "to be good at", "v1", "vi")),
			entry(1591930, "気づく", "きづく", sense(0, "to notice", "v5k", "vi")),
			entry(1509930, "選ぶ", "えらぶ", sense(0, "to choose", "v5b", "vt")),
			entry(1185200, "遅い", "おそい", sense(0, "slow", "adj-i")),
			entry(1593180, "連れる", "つれる", sense(0, "to take (someone) with one", "v1", "vt")),
			entry(1315720, "持つ", "もつ", sense(0, "to hold", "v5t", "vt")),
			entry(1294490, "感じる", "かんじる", sense(0, "to feel", "v1", "vt")),
			entry(2847957, "うっせぇ", "うっせぇ", sense(0, "shut up!", "int")),
			entry(2069620, "はぁ", "はぁ", sense(0, "yes", "int"), sense(1, "ha!", "int"), sense(2, "what?", "int")),
			entry(1275640, "口", "くち", sense(0, "mouth", "n")),
			entry(1609420, "塞げる", "ふさげる", sense(0, "to close up", "v1", "vt")),
			entry(1214920, "詰まる", "つまる", sense(0, "to be packed", "v5r", "vi")),
			entry(1214980, "つまらない", "つまらない", sense(0, "dull", "adj-i")),
			entry(1595900, "変わる", "かわる", sense(0, "to change", "v5r", "vi")),
			entry(1595910, "変わらない", "かわらない", sense(0, "constant", "adj-i")),
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
		{text: "気がつく", want: []string{"気がつく 1591050/0 v5k"}},
		{text: "気が付いた", want: []string{"気が付く 1591050/0 v5k"}},
		{text: "なんだ", want: []string{"なん 1188490/0 ", "だ 0/0 "}},
		{text: "1番", want: []string{"一番 1165970/0 "}},
		{text: "迷惑かけた", want: []string{"迷惑をかける 1911730/0 v1"}},
		{text: "なんもねえ", want: []string{"なんも 1188500/0 ", "ない 1529520/0 adj-i"}},
		{text: "はんぱねえよ", want: []string{"はんぱない 2746990/0 adj-i"}},
		{text: "いつかきっと", want: []string{"いつか 1188790/0 ", "きっと 0/0 "}},
		{text: "全てが", want: []string{"全て 1595730/0 ", "が 0/0 "}},
		{text: "全て忘れて", want: []string{"全て 1595730/1 ", "忘れる 0/0 v1"}},
		{text: "サン！ヨン！ゼロワン！", want: []string{
			"サン 2747950/0 ", "ヨン 1579470/0 ", "ゼロ 2839962/0 ", "ワン 1149130/0 ",
		}},
		{text: "サン！サン！ゼロ", want: []string{"サン 2747950/0 ", "サン 2747950/0 ", "ゼロ 2839962/0 "}},
		{text: "サンが昇る", want: []string{"サン 1058210/0 ", "が 0/0 ", "昇る 0/0 v5r"}},
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
		{text: "がんばれ がんばれ", want: []string{
			"がんばれ がんばる/がんばる content v5r [imperative]", "がんばれ がんばる/がんばる content v5r [imperative]",
		}},
		{text: "がんばれよ 俺ら", want: []string{
			"がんばれよ がんばる/がんばる content v5r [imperative]", "俺ら 俺ら/おれら content",
		}},
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
	hollow := Entry{ID: 9, Forms: []Form{{Text: "洞", Reading: "ほら", Common: true}, {Text: "ほら", Reading: "ほら", Common: true}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}}}}
	boasting := Entry{ID: 10, Forms: []Form{{Text: "法螺", Reading: "ほら"}, {Text: "ホラ", Reading: "ほら"}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}}}}
	difficulty := Entry{ID: 11, Forms: []Form{{Text: "なん", Reading: "なん", Common: true}},
		Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"n"}}}}
	what := Entry{ID: 12, Forms: []Form{{Text: "なん", Reading: "なん"}}, Senses: []Sense{{Index: 0, PartsOfSpeech: []string{"pn"}}}}
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
		{name: "the spelling as written over a common one", entries: []Entry{hollow, boasting}, cat: catNoun,
			form: "ホラ", wantID: 10},
		{name: "a pronoun sense for a pronoun", entries: []Entry{difficulty, what}, cat: catPronoun, form: "なん",
			wantID: 12},
		{name: "a noun sense when a pronoun has no pronoun sense", entries: []Entry{ichidan, difficulty},
			cat: catPronoun, form: "なん", wantID: 11},
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

func TestNumbersInDigitsAreReadLikeTheirKanji(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text        string
		wantNumber  string
		wantCounter string
		wantLemma   string
	}{
		{text: "1番", wantNumber: "いち", wantCounter: "ばん", wantLemma: "いちばん"},
		{text: "2人", wantNumber: "ふたり", wantCounter: "", wantLemma: "ふたり"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := analyzedLine(t, dict, tt.text)
			if len(line.Analysis) != 1 || len(line.Tokens) != 2 {
				t.Fatalf("words = %+v over %d tokens, want one word over two", line.Analysis, len(line.Tokens))
			}
			got := []string{line.Tokens[0].Reading, line.Tokens[1].Reading, line.Analysis[0].LemmaReading}
			if want := []string{tt.wantNumber, tt.wantCounter, tt.wantLemma}; !slices.Equal(got, want) {
				t.Errorf("readings of %q (number, counter, lemma) = %q, want %q", tt.text, got, want)
			}
		})
	}
}

func TestLookupEntriesDescribesEachEntryOnce(t *testing.T) {
	t.Parallel()

	got, err := LookupEntries(t.Context(), newFakeDictionary(t), []int64{1593670, 1245290, 1593670, 999})
	if err != nil {
		t.Fatalf("LookupEntries: %v", err)
	}
	if len(got) != 2 || got[0].EntryID != 1593670 || got[1].EntryID != 1245290 {
		t.Fatalf("LookupEntries = %+v, want 探す then 空, once each, and nothing for an unknown ID", got)
	}
	want := Meaning{Index: 0, Glosses: []string{"to search for"},
		PartsOfSpeech: []string{"Godan verb with 'su' ending", "transitive verb"}, Notes: []string{}}
	if !reflect.DeepEqual(got[0].Senses[0], want) || !got[0].Installed || got[0].Attribution == "" {
		t.Errorf("探す = %+v, want installed with the credit and %+v", got[0], want)
	}
}

func TestLookupEntriesWithoutTheDictionary(t *testing.T) {
	t.Parallel()

	got, err := LookupEntries(t.Context(), noMeanings(t), []int64{1593670})
	if err != nil || len(got) != 0 || got == nil {
		t.Errorf("LookupEntries = %+v, %v, want an empty list", got, err)
	}
}

func TestLookupEntriesRejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ids  []int64
	}{
		{name: "zero", ids: []int64{0}},
		{name: "negative", ids: []int64{1593670, -1}},
		{name: "too many", ids: make([]int64, maxLookupEntries+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := LookupEntries(t.Context(), newFakeDictionary(t), tt.ids)
			var invalid *fault.InvalidError
			if !errors.As(err, &invalid) || invalid.Field != "ids" {
				t.Errorf("LookupEntries(%s) error = %v, want invalid ids", tt.name, err)
			}
		})
	}
}

func TestSpokenFormsBecomeTheWordsTheyStandFor(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text       string
		want       []string
		wantRomaji string
	}{
		{text: "ググってる", want: []string{"ググってる ググる/ぐぐる content v5r [te-form teru]"}, wantRomaji: "gugutteru"},
		{text: "ググった", want: []string{"ググった ググる/ぐぐる content v5r [past]"}, wantRomaji: "gugutta"},
		{text: "テヘペロって", want: []string{"テヘペロ テヘペロ/てへぺろ content", "って って/って particle tte-quote"},
			wantRomaji: "tehepero tte"},
		{text: "指が臭え", want: []string{
			"指 指/ゆび content", "が が/が particle ga-subject", "臭え 臭い/くさい content adj-i",
		}, wantRomaji: "yubi ga kusee"},
		{text: "仕方ねえ", want: []string{"仕方ねえ 仕方ない/しかたない content adj-i"}, wantRomaji: "shikata nee"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := analyzedLine(t, dict, tt.text)
			if got := describeWords(line.Tokens, line.Analysis); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
			if got := romaji(line.Tokens); got != tt.wantRomaji {
				t.Errorf("romaji of %q = %q, want %q", tt.text, got, tt.wantRomaji)
			}
		})
	}
}

func TestLyricPatternsReadAsTheyAreMeant(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text string
		want []string
	}{
		{text: "何の分け隔てもない", want: []string{
			"何 何/なに content 1188490", "の の/の particle no-possession 0", "分け隔て 分け隔て/わけへだて content 1601440",
			"も も/も particle mo-also 0", "ない ない/ない content adj-i 1529520",
		}},
		{text: "まとめてかかってきなさい", want: []string{
			"まとめて まとめて/まとめて content 1525360",
			"かかってきなさい かかる/かかる content v5r [te-form te-kuru imperative] 2084130",
		}},
		{text: "かかってこい", want: []string{
			"かかってこい かかる/かかる content v5r [te-form te-kuru imperative] 2084130",
		}},
		{text: "そういうわけで", want: []string{
			"そういう そういう/そういう content 0", "わけ 訳/わけ content 1600820", "で だ/だ auxiliary 0",
		}},
		{text: "選びな", want: []string{"選びな 選ぶ/えらぶ content v5b [imperative] 1509930"}},
		{text: "遅くはねーから", want: []string{
			"遅く 遅い/おそい content adj-i [adverbial] 1185200", "は は/は particle wa-topic 0",
			"ねー ない/ない content adj-i 1529520", "から から/から particle kara-because 0",
		}},
		{text: "知ったこっちゃない", want: []string{
			"知ったこっちゃない 知ったこっちゃない/しったこっちゃない content 2137720",
		}},
		{text: "肩落とすこたない", want: []string{
			"肩 肩/かた content 1303980", "落とす 落とす/おとす content v5s 1374630", "こた こと/こと content 1313580",
			"ない ない/ない content adj-i 1529520",
		}},
		{text: "そこへ行けた", want: []string{
			"そこ そこ/そこ content 0", "へ へ/へ particle he-direction 0",
			"行けた 行く/いく content v5k-s [potential past] 1578850",
		}},
		{text: "気づけん", want: []string{"気づけん 気づく/きづく content v5k [potential negative] 1591930"}},
		{text: "連れてって", want: []string{
			"連れてって 連れる/つれる content v1 [te-form te-iku te-form] 1593180",
		}},
		{text: "持っていける", want: []string{"持っていける 持つ/もつ content v5t [te-form] 1315720"}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := analyzedLine(t, dict, tt.text)
			got := describeWords(line.Tokens, line.Analysis)
			for i, w := range line.Analysis {
				got[i] += " " + strconv.FormatInt(w.EntryID, 10)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
		})
	}
}

// recutLine analyses text as a song's line is: kagome's tokens, cut again where it couldn't read the kana, then words.
func recutLine(t *testing.T, dict Dictionary, text string) Line {
	t.Helper()
	m := newMeanings(dict)
	tokens, err := m.recut(t.Context(), analyzeText(t, morph.New(), text))
	if err != nil {
		t.Fatalf("recut(%q): %v", text, err)
	}
	line := Line{Text: text, Tokens: tokens}
	if err := m.analyzeWords(t.Context(), &line); err != nil {
		t.Fatalf("analyzeWords(%q): %v", text, err)
	}
	return line
}

func TestKanaKagomeCantReadIsCutAgain(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text       string
		want       []string
		wantRomaji string
	}{
		{text: "はぁ？うっせぇうっせぇうっせぇわ", want: []string{
			"はぁ はぁ/はぁ content 2069620/2", "うっせぇ うっせぇ/うっせぇ content 2847957/0",
			"うっせぇ うっせぇ/うっせぇ content 2847957/0", "うっせぇ うっせぇ/うっせぇ content 2847957/0",
			"わ わ/わ particle wa-feeling 0/0",
		}, wantRomaji: "haa? ussee ussee ussee wa"},
		{text: "くせぇ口塞げや", want: []string{
			"くせぇ くさい/くさい content adj-i 1333150/0", "口 口/くち content 1275640/0",
			"塞げ 塞げる/ふさげる content v1 [continuative] 1609420/0", "や や/や auxiliary 0/0",
		}, wantRomaji: "kusee kuchi fusageya"},
		{text: "知ったこっちゃない", want: []string{
			"知ったこっちゃない 知ったこっちゃない/しったこっちゃない content 2137720/0",
		}, wantRomaji: "shitta kotchi yanai"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := recutLine(t, dict, tt.text)
			got := describeWords(line.Tokens, line.Analysis)
			for i, w := range line.Analysis {
				got[i] += " " + strconv.FormatInt(w.EntryID, 10) + "/" + strconv.Itoa(w.SenseIndex)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
			if got := romaji(line.Tokens); got != tt.wantRomaji {
				t.Errorf("romaji of %q = %q, want %q", tt.text, got, tt.wantRomaji)
			}
		})
	}
}

func TestKatakanaKagomeCantReadIsLeftAlone(t *testing.T) {
	t.Parallel()

	m := newMeanings(newFakeDictionary(t))
	tokens := []Token{{Surface: "エイデイ", Reading: "えいでい", POS: morph.Noun}}
	got, err := m.recut(t.Context(), tokens)
	if err != nil || !reflect.DeepEqual(got, tokens) {
		t.Errorf("recut = %+v, %v; want katakana left as kagome read it", got, err)
	}
}

func TestSpokenNegativesReadAsTheirAdjective(t *testing.T) {
	t.Parallel()

	dict := newFakeDictionary(t)
	tests := []struct {
		text string
		want []string
	}{
		{text: "つまらねぇ", want: []string{"つまらねぇ つまらない/つまらない content adj-i"}},
		// A plain negative stays the verb's: 変わらない is "not change", though the dictionary has "constant".
		{text: "変わらない", want: []string{"変わらない 変わる/かわる content v5r [negative]"}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			line := recutLine(t, dict, tt.text)
			if got := describeWords(line.Tokens, line.Analysis); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("words of %q\n got  %q\n want %q", tt.text, got, tt.want)
			}
		})
	}
}
