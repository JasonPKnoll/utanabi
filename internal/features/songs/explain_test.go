package songs

import (
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

func presentLine(t *testing.T, dict Dictionary, text string) Line {
	t.Helper()
	song, err := present(t.Context(), dict, Song{Lines: []Line{analyzedLine(t, dict, text)}})
	if err != nil {
		t.Fatalf("present: %v", err)
	}
	return song.Lines[0]
}

func withMeanings(t *testing.T) Dictionary {
	t.Helper()
	return newFakeDictionary(t)
}

func TestLineGlossIsWordByWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dict func(t *testing.T) Dictionary
		text string
		want string
	}{
		{name: "with meanings", dict: withMeanings,
			text: "夜の空に星が光る", want: "night · 's · sky · in · star · (subject) · shine"},
		{name: "without meanings", dict: noMeanings, text: "夜の空に星が光る",
			want: "'s · in · (subject)"},
		{name: "a conjugated verb", dict: withMeanings,
			text: "君の声を探している", want: "you · 's · voice · (object) · search for"},
		{name: "a copula and a set phrase", dict: noMeanings, text: "夢じゃない、君だ",
			want: "is not · is"},
		{name: "only punctuation", dict: noMeanings, text: "、！", want: ""},
		{name: "a negative says not", dict: withMeanings, text: "消えない", want: "not disappear"},
		{name: "の after a verb is the act", dict: noMeanings, text: "死ぬのがいい", want: "(-ing) · (subject)"},
		{name: "の after an adjective is the one", dict: noMeanings, text: "ダサいのは", want: "(one) · (topic)"},
		{name: "の before よ explains", dict: noMeanings, text: "いいのよ", want: "(it's that) · (!)"},
		{name: "なんて after a clause", dict: noMeanings, text: "会えるなんて", want: "(to think…)"},
		{name: "よ calling out", dict: noMeanings, text: "星よ光れ", want: "(calling) · 光る"},
		{name: "もう before a negative", dict: noMeanings, text: "もう要らない", want: "anymore · 要る"},
		{name: "もう with no negative", dict: noMeanings, text: "もう行く", want: ""},
		{name: "ない standing alone", dict: withMeanings, text: "なんもねえ", want: "anything · not there"},
		{name: "an adjective keeps its to", dict: withMeanings, text: "はんぱねえ", want: "to a great extent"},
		{name: "a phrase with its particle dropped", dict: withMeanings, text: "迷惑かける", want: "cause trouble"},
		{name: "ん between nouns", dict: noMeanings, text: "胸んなか", want: "'s"},
		{name: "ない after a く form says not", dict: withMeanings, text: "遅くはねーから", want: "slow · (topic) · not · because"},
		{name: "ない after a く form and も says not", dict: withMeanings, text: "感じたくもない",
			want: "feel · also · not"},
		{name: "a potential negative says can't", dict: withMeanings, text: "気づけん", want: "can't notice"},
		{name: "a potential past says could", dict: withMeanings, text: "そこへ行けた", want: "toward · could go"},
		{name: "a potential says can", dict: withMeanings, text: "会える", want: "can meet"},
		{name: "a te form adverb and a come command", dict: withMeanings, text: "まとめてかかってきなさい",
			want: "all together · bring it on!"},
		{name: "a phrase joined around an unknown piece", dict: withMeanings, text: "知ったこっちゃない",
			want: "I don't give a damn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := presentLine(t, tt.dict(t), tt.text).Gloss; got != tt.want {
				t.Errorf("gloss of %q = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestWordsCarryTheirMeaningAndGrammar(t *testing.T) {
	t.Parallel()

	line := presentLine(t, newFakeDictionary(t), "君の声を探している")
	if len(line.Words) != 5 {
		t.Fatalf("got %d words, want 5", len(line.Words))
	}
	shorts := make([]string, len(line.Words))
	for i, w := range line.Words {
		shorts[i] = w.Short
	}
	if want := []string{"you", "'s", "voice", "(object)", "search for"}; !reflect.DeepEqual(shorts, want) {
		t.Errorf("shorts = %q, want %q (each word's part of the line gloss)", shorts, want)
	}
	verb := line.Words[4]
	sense := 0
	want := Word{
		FirstToken: 4, LastToken: 6, Text: "探している", Lemma: "探す", LemmaReading: "さがす", LemmaRomaji: "sagasu",
		Kind: WordContent, ConjClass: "v5s", EntryID: 1593670, SenseIndex: &sense, Gloss: "to search for",
		Short: "search for", Form: "探す, in the 〜ている form",
	}
	got := verb
	got.Steps = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("verb = %+v, want %+v", got, want)
	}
	if len(verb.Steps) != 2 || verb.Steps[0].ID != "te-form" || verb.Steps[1].ID != "teiru" ||
		verb.Steps[1].Title == "" || verb.Steps[1].Meaning == "" {
		t.Errorf("steps = %+v, want te-form and teiru with their titles and meanings", verb.Steps)
	}

	particle := line.Words[1]
	if particle.Grammar == nil || particle.Grammar.ID != "no-possession" || particle.Grammar.Explanation == "" {
		t.Errorf("の grammar = %+v, want no-possession with its explanation", particle.Grammar)
	}
	if particle.SenseIndex != nil || particle.EntryID != 0 {
		t.Errorf("の has entry %d sense %v, want none", particle.EntryID, particle.SenseIndex)
	}
}

func TestPiecesCutTheLineAtWordEdges(t *testing.T) {
	t.Parallel()

	line := presentLine(t, noMeanings(t), "「君」と、星")
	want := []LinePiece{
		{Ruby: []RubyPart{{Text: "「"}}, Word: -1},
		{Ruby: []RubyPart{{Text: "君", Reading: "きみ"}}, Word: 0},
		{Ruby: []RubyPart{{Text: "」"}}, Word: -1},
		{Ruby: []RubyPart{{Text: "と"}}, Word: 1},
		{Ruby: []RubyPart{{Text: "、"}}, Word: -1},
		{Ruby: []RubyPart{{Text: "星", Reading: "ほし"}}, Word: 2},
	}
	if !reflect.DeepEqual(line.Pieces, want) {
		t.Errorf("pieces = %+v, want %+v", line.Pieces, want)
	}
}

func TestALineWithoutTokensIsOnePiece(t *testing.T) {
	t.Parallel()

	song, err := present(t.Context(), noMeanings(t), Song{Lines: []Line{{Text: "星が光る"}}})
	if err != nil {
		t.Fatalf("present: %v", err)
	}
	line := song.Lines[0]
	if want := []LinePiece{{Ruby: []RubyPart{{Text: "星が光る"}}, Word: -1}}; !reflect.DeepEqual(line.Pieces, want) {
		t.Errorf("pieces = %+v, want %+v", line.Pieces, want)
	}
	if line.Words == nil || len(line.Words) != 0 || line.Gloss != "" {
		t.Errorf("words = %+v, gloss = %q, want an empty list and no gloss", line.Words, line.Gloss)
	}
}

func TestWordsThatDontFitTheTokensAreLeftOut(t *testing.T) {
	t.Parallel()

	tokens := []Token{{Surface: "星", Reading: "ほし", POS: morph.Noun}}
	line := Line{Text: "星", Tokens: tokens, Analysis: []WordAnalysis{
		{FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "星"},
		{FirstToken: 0, LastToken: 3, Kind: WordContent, Lemma: "星空"},
	}}
	words, _, gloss := explainLine(noMeanings(t), nil, line)
	if len(words) != 1 || words[0].Lemma != "星" || gloss != "" {
		t.Errorf("words = %+v, gloss = %q, want only the word that fits", words, gloss)
	}
}

func TestFormSentence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lemma string
		steps []string
		want  string
	}{
		{lemma: "探す", steps: []string{"te-form", "teiru"}, want: "探す, in the 〜ている form"},
		{lemma: "待つ", steps: []string{"te-form", "teru"}, want: "待つ, in the 〜てる form"},
		{lemma: "光る", steps: []string{"past"}, want: "光る, in the 〜た form"},
		{lemma: "食べる", steps: []string{"causative", "passive", "negative", "past"},
			want: "食べる, in the causative + passive + 〜ない + 〜た form"},
		{lemma: "会う", steps: []string{"potential", "negative"}, want: "会う, in the potential + 〜ない form"},
		{lemma: "降る", steps: []string{"te-form"}, want: "降る, in the 〜て form"},
		{lemma: "星", steps: nil, want: ""},
	}
	for _, tt := range tests {
		if got := formSentence(tt.lemma, tt.steps); got != tt.want {
			t.Errorf("formSentence(%s, %v) = %q, want %q", tt.lemma, tt.steps, got, tt.want)
		}
	}
}

func TestShortGloss(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{in: "star (usu. excluding the Sun)", want: "star"},
		{in: "(the) sky", want: "sky"},
		{in: "to shine", want: "to shine"},
		{in: "(something)", want: "(something)"},
		{in: "a very long gloss that keeps going on and on well past what fits under a single word", want: "a very long gloss that keeps going on and on well past what…"},
	}
	for _, tt := range tests {
		if got := shortGloss(tt.in); got != tt.want {
			t.Errorf("shortGloss(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetSongFindsWordsForLinesSavedWithTokensOnly(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	tokens := analyzeText(t, morph.New(), "夜の空に星が光る")
	stored := make([]Token, len(tokens))
	for i, tok := range tokens {
		stored[i] = Token{Surface: tok.Surface, Reading: tok.Reading, POS: tok.POS}
	}
	repo.songs[1] = Song{ID: 1, Title: "星の歌", Lines: []Line{{Text: "夜の空に星が光る", Tokens: stored}}}

	got, err := GetSong(t.Context(), repo, NewMorph(morph.New()), newFakeDictionary(t), 1)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if got.Lines[0].Gloss != "night · 's · sky · in · star · (subject) · shine" {
		t.Errorf("gloss = %q", got.Lines[0].Gloss)
	}
	if saved := repo.songs[1].Lines[0].Analysis; len(saved) != 7 || saved[6].Lemma != "光る" {
		t.Errorf("saved words = %+v, want the line's 7 words", saved)
	}
}

func TestGetSongLeavesWordsWhenTheTokensNoLongerMatch(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	stored := []Token{{Surface: "夜の", POS: morph.Noun}, {Surface: "空", POS: morph.Noun}}
	repo.songs[1] = Song{ID: 1, Title: "星の歌", Lines: []Line{{Text: "夜の空", Tokens: stored}}}

	got, err := GetSong(t.Context(), repo, NewMorph(morph.New()), noMeanings(t), 1)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if len(got.Lines[0].Words) != 0 || repo.tokenSaves != 0 {
		t.Errorf("words = %+v and %d saves, want none for tokens the analyzer no longer gives", got.Lines[0].Words,
			repo.tokenSaves)
	}
}

func TestGetSongFillsMeaningsFoundWithoutTheDictionary(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	tokens := []Token{{Surface: "星", Reading: "ほし", POS: morph.Noun}, {Surface: "空", Reading: "そら", POS: morph.Noun}}
	repo.songs[1] = Song{ID: 1, Title: "星の歌", Lines: []Line{{Text: "星空", Tokens: tokens, Analysis: []WordAnalysis{
		{ID: 10, FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "星", LemmaReading: "ほし",
			Source: AnalysisAuto},
		{ID: 11, FirstToken: 1, LastToken: 1, Kind: WordContent, Lemma: "空", LemmaReading: "そら",
			Source: AnalysisUser},
	}}}}

	got, err := GetSong(t.Context(), repo, fakeAnalyzer{}, newFakeDictionary(t), 1)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if len(repo.meaningSaves) != 1 || repo.meaningSaves[0].ID != 10 || repo.meaningSaves[0].EntryID != 1376310 {
		t.Errorf("saved meanings = %+v, want 星 (word 10) as entry 1376310, and the word you made left alone",
			repo.meaningSaves)
	}
	if got.Lines[0].Words[0].Gloss != "star" {
		t.Errorf("gloss = %q, want star", got.Lines[0].Words[0].Gloss)
	}

	repo.meaningSaves = nil
	if _, err := GetSong(t.Context(), repo, fakeAnalyzer{}, noMeanings(t), 1); err != nil {
		t.Fatalf("GetSong without meanings: %v", err)
	}
	if len(repo.meaningSaves) != 0 {
		t.Errorf("saved %d meanings without the dictionary, want none", len(repo.meaningSaves))
	}
}

func TestWordGlossLeavesOutAWordThatOnlyRepeatsItself(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word Word
		want string
	}{
		{name: "no meaning, same form", word: Word{Text: "げんまん", Lemma: "げんまん", Kind: WordContent}, want: ""},
		{name: "no meaning, other form", word: Word{Text: "探して", Lemma: "探す", Kind: WordContent}, want: "探す"},
		{name: "a meaning", word: Word{Text: "星", Lemma: "星", Kind: WordContent, Gloss: "star"}, want: "star"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := wordGloss(tt.word, glossContext{}); got != tt.want {
				t.Errorf("wordGloss(%q) = %q, want %q", tt.word.Text, got, tt.want)
			}
		})
	}
}
