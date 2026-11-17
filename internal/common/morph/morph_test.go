package morph_test

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

type brief struct {
	Surface string
	Reading string
	POS     morph.POS
}

func analyze(t *testing.T, a *morph.Analyzer, text string) []morph.Token {
	t.Helper()
	tokens, err := a.Analyze(text)
	if err != nil {
		t.Fatalf("Analyze(%q): %v", text, err)
	}
	return tokens
}

func TestAnalyze(t *testing.T) {
	t.Parallel()

	a := morph.New()
	tests := []struct {
		text string
		want []brief
	}{
		{text: "君の声を探している", want: []brief{
			{"君", "きみ", morph.Pronoun}, {"の", "の", morph.Particle}, {"声", "こえ", morph.Noun},
			{"を", "を", morph.Particle}, {"探し", "さがし", morph.Verb}, {"て", "て", morph.Particle},
			{"いる", "いる", morph.Verb},
		}},
		{text: "夜の空に星が光る", want: []brief{
			{"夜", "よる", morph.Noun}, {"の", "の", morph.Particle}, {"空", "そら", morph.Noun},
			{"に", "に", morph.Particle}, {"星", "ほし", morph.Noun}, {"が", "が", morph.Particle},
			{"光る", "ひかる", morph.Verb},
		}},
		{text: "食べさせられなかった", want: []brief{
			{"食べ", "たべ", morph.Verb}, {"させ", "させ", morph.Suffix}, {"られ", "られ", morph.Suffix},
			{"なかっ", "なかっ", morph.Auxiliary}, {"た", "た", morph.Auxiliary},
		}},
		{text: "会いに行こう", want: []brief{
			{"会い", "あい", morph.Verb}, {"に", "に", morph.Particle}, {"行こ", "いこ", morph.Verb},
			{"う", "う", morph.Auxiliary},
		}},
		{text: "待ってる", want: []brief{{"待っ", "まっ", morph.Verb}, {"てる", "てる", morph.Verb}}},
		{text: "love you 君", want: []brief{
			{"love", "", morph.Noun}, {" ", "", morph.Symbol}, {"you", "", morph.Noun}, {" ", "", morph.Symbol},
			{"君", "きみ", morph.Pronoun},
		}},
		{text: "「さよなら」、また明日！", want: []brief{
			{"「", "", morph.Symbol}, {"さよなら", "さよなら", morph.Interjection}, {"」", "", morph.Symbol},
			{"、", "", morph.Symbol}, {"また", "また", morph.Conjunction}, {"明日", "あした", morph.Noun},
			{"！", "", morph.Symbol},
		}},
		{text: "ラーメンとケータイ", want: []brief{
			{"ラーメン", "らーめん", morph.Noun}, {"と", "と", morph.Particle}, {"ケータイ", "けーたい", morph.Noun},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			tokens := analyze(t, a, tt.text)
			got := make([]brief, len(tokens))
			for i, tok := range tokens {
				got[i] = brief{tok.Surface, tok.Reading, tok.POS}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Analyze(%q)\n got  %v\n want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestAnalyzeKeepsTheIPAFields(t *testing.T) {
	t.Parallel()

	tokens := analyze(t, morph.New(), "君を探している")
	want := []morph.Token{
		{Surface: "君", Reading: "きみ", POS: morph.Pronoun, POSDetail: "代名詞", BaseForm: "君", Pronunciation: "キミ"},
		{Surface: "を", Reading: "を", POS: morph.Particle, POSDetail: "格助詞", BaseForm: "を", Pronunciation: "ヲ"},
		{Surface: "探し", Reading: "さがし", POS: morph.Verb, POSDetail: "自立", BaseForm: "探す", ConjType: "五段・サ行",
			ConjForm: "連用形", Pronunciation: "サガシ"},
		{Surface: "て", Reading: "て", POS: morph.Particle, POSDetail: "接続助詞", BaseForm: "て", Pronunciation: "テ"},
		{Surface: "いる", Reading: "いる", POS: morph.Verb, POSDetail: "非自立", BaseForm: "いる", ConjType: "一段",
			ConjForm: "基本形", Pronunciation: "イル"},
	}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("got  %+v\nwant %+v", tokens, want)
	}
}

func TestAnalyzeHandlesWordsTheDictionaryDoesntKnow(t *testing.T) {
	t.Parallel()

	tokens := analyze(t, morph.New(), "love")
	want := []morph.Token{{Surface: "love", POS: morph.Noun, POSDetail: "固有名詞"}}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("got %+v, want %+v", tokens, want)
	}
}

func TestAnalyzeSurfacesJoinBackIntoTheText(t *testing.T) {
	t.Parallel()

	a := morph.New()
	for _, text := range []string{
		"君の声を探している", "love you 君", "　夜の空、星が光る！！", "I'm 2人で…", "ﾗｲﾌﾞ", "♪ ララ ♪", "",
	} {
		var b strings.Builder
		for _, tok := range analyze(t, a, text) {
			b.WriteString(tok.Surface)
		}
		if got := b.String(); got != text {
			t.Errorf("surfaces of %q join into %q", text, got)
		}
	}
}

func TestAnalyzeIsSafeForConcurrentCallers(t *testing.T) {
	t.Parallel()

	a := morph.New()
	var wg sync.WaitGroup
	results := make([]int, 8)
	for i := range results {
		wg.Go(func() {
			tokens, err := a.Analyze("夜の空に星が光る")
			if err != nil {
				t.Errorf("Analyze: %v", err)
				return
			}
			results[i] = len(tokens)
		})
	}
	wg.Wait()
	for i, n := range results {
		if n != 7 {
			t.Errorf("caller %d got %d tokens, want 7", i, n)
		}
	}
}
