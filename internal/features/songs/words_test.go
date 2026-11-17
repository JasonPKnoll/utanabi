package songs

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// describeWords writes each word as "surface lemma/reading kind class grammar [steps]", leaving out what is empty.
func describeWords(tokens []Token, words []WordAnalysis) []string {
	out := make([]string, len(words))
	for i, w := range words {
		var surface strings.Builder
		for _, t := range tokens[w.FirstToken : w.LastToken+1] {
			surface.WriteString(t.Surface)
		}
		parts := []string{surface.String(), w.Lemma + "/" + w.LemmaReading, string(w.Kind)}
		if w.ConjClass != "" {
			parts = append(parts, w.ConjClass)
		}
		if w.GrammarPointID != "" {
			parts = append(parts, w.GrammarPointID)
		}
		if len(w.Steps) > 0 {
			parts = append(parts, fmt.Sprint(w.Steps))
		}
		out[i] = strings.Join(parts, " ")
	}
	return out
}

func analyzeText(t *testing.T, a *morph.Analyzer, text string) []Token {
	t.Helper()
	tokens, err := NewMorph(a).Analyze(t.Context(), text)
	if err != nil {
		t.Fatalf("Analyze(%q): %v", text, err)
	}
	return tokens
}

func TestFindWords(t *testing.T) {
	t.Parallel()

	a := morph.New()
	tests := []struct {
		text string
		want []string
	}{
		{text: "君の声を探している", want: []string{
			"君 君/きみ content", "の の/の particle no-possession", "声 声/こえ content", "を を/を particle wo-object",
			"探している 探す/さがす content v5s [te-form teiru]",
		}},
		{text: "夜の空に星が光る", want: []string{
			"夜 夜/よる content", "の の/の particle no-possession", "空 空/そら content", "に に/に particle ni-place",
			"星 星/ほし content", "が が/が particle ga-subject", "光る 光る/ひかる content v5r",
		}},
		{text: "食べさせられなかった", want: []string{
			"食べさせられなかった 食べる/たべる content v1 [causative passive negative past]",
		}},
		{text: "会いに行こう", want: []string{
			"会い 会う/あう content v5u [continuative]", "に に/に particle ni-purpose",
			"行こう 行く/いく content v5k-s [volitional]",
		}},
		{text: "光った", want: []string{"光った 光る/ひかる content v5r [past]"}},
		{text: "美しかった", want: []string{"美しかった 美しい/うつくしい content adj-i [past]"}},
		{text: "行こう", want: []string{"行こう 行く/いく content v5k-s [volitional]"}},
		{text: "会えない", want: []string{"会えない 会える/あえる content v1 [negative]"}},
		{text: "待ってる", want: []string{"待ってる 待つ/まつ content v5t [te-form teru]"}},
		{text: "消えぬ", want: []string{"消えぬ 消える/きえる content v1 [nu]"}},
		{text: "知らず", want: []string{"知らず 知る/しる content v5r [zu]"}},
		{text: "書かれた", want: []string{"書かれた 書く/かく content v5k [passive past]"}},
		{text: "着れる", want: []string{"着れる 着る/きる content v1 [ra-nuki]"}},
		{text: "忘れてしまった", want: []string{"忘れてしまった 忘れる/わすれる content v1 [te-form te-shimau past]"}},
		{text: "食べちゃう", want: []string{"食べちゃう 食べる/たべる content v1 [chau]"}},
		{text: "行ったら", want: []string{"行ったら 行く/いく content v5k-s [conditional-tara]"}},
		{text: "降れば", want: []string{"降れば 降る/ふる content v5r [conditional-ba]"}},
		{text: "言いません", want: []string{"言いません 言う/いう content v5u [polite-masu negative]"}},
		{text: "行きましょう", want: []string{"行きましょう 行く/いく content v5k-s [polite-masu volitional]"}},
		{text: "来た", want: []string{"来た 来る/くる content vk [past]"}},
		{text: "走れ", want: []string{"走れ 走る/はしる content v5r [imperative]"}},
		{text: "早く", want: []string{"早く 早い/はやい content adj-i [adverbial]"}},
		{text: "美しくない", want: []string{"美しくない 美しい/うつくしい content adj-i [negative]"}},
		{text: "君に会いたい", want: []string{
			"君 君/きみ content", "に に/に particle ni-target", "会いたい 会う/あう content v5u [tai-want]",
		}},
		{text: "夜に会う", want: []string{
			"夜 夜/よる content", "に に/に particle ni-time", "会う 会う/あう content v5u",
		}},
		{text: "東京に行く", want: []string{
			"東京 東京/とうきょう content", "に に/に particle ni-destination", "行く 行く/いく content v5k-s",
		}},
		{text: "大人になる", want: []string{
			"大人 大人/おとな content", "に に/に particle ni-change", "なる なる/なる content v5r",
		}},
		{text: "静かな夜", want: []string{
			"静かな 静か/しずか content adj-na [na-attributive]", "夜 夜/よる content",
		}},
		{text: "好きだから", want: []string{
			"好きだ 好き/すき content adj-na", "から から/から particle kara-because",
		}},
		{text: "東京から", want: []string{"東京 東京/とうきょう content", "から から/から particle kara-from"}},
		{text: "綺麗だな", want: []string{"綺麗だ 綺麗/きれい content adj-na", "な な/な particle na-feeling"}},
		{text: "夢じゃない", want: []string{"夢 夢/ゆめ content", "じゃない じゃない/じゃない expression ja-nai"}},
		{text: "夢じゃなかった", want: []string{
			"夢 夢/ゆめ content", "じゃなかった じゃない/じゃない expression ja-nai [past]",
		}},
		{text: "行くんだ", want: []string{"行く 行く/いく content v5k-s", "んだ んだ/んだ expression n-da"}},
		{text: "走るのが好き", want: []string{
			"走る 走る/はしる content v5r", "の の/の particle no-nominalizer", "が が/が particle ga-subject",
			"好き 好き/すき content adj-na",
		}},
		{text: "勉強する", want: []string{"勉強する 勉強する/べんきょうする content vs"}},
		{text: "雨でしょう", want: []string{"雨 雨/あめ content", "でしょう です/です auxiliary darou"}},
		{text: "君だ", want: []string{"君 君/きみ content", "だ だ/だ auxiliary"}},
		{text: "「君」と", want: []string{"君 君/きみ content", "と と/と particle to-quote"}},
		{text: "君と僕", want: []string{
			"君 君/きみ content", "と と/と particle to-and", "僕 僕/ぼく content",
		}},
		{text: "何か", want: []string{"何 何/なに content", "か か/か particle ka-some"}},
		{text: "雨が降っても", want: []string{
			"雨 雨/あめ content", "が が/が particle ga-subject", "降って 降る/ふる content v5r [te-form]",
			"も も/も particle te-mo",
		}},
		{text: "行くなよ", want: []string{
			"行く 行く/いく content v5k-s", "な な/な particle na-prohibition", "よ よ/よ particle yo-emphasis",
		}},
		{text: "田中さんに会う", want: []string{
			"田中さん 田中さん/たなかさん content", "に に/に particle ni-target", "会う 会う/あう content v5u",
		}},
		{text: "学校で", want: []string{"学校 学校/がっこう content", "で で/で particle de-place"}},
		{text: "love you 君", want: []string{"love love/ content", "you you/ content", "君 君/きみ content"}},
		{text: "、！", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			tokens := analyzeText(t, a, tt.text)
			got := describeWords(tokens, findWords(tokens))
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findWords(%q)\n got  %q\n want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestFindWordsCoversEveryTokenButPunctuationOnce(t *testing.T) {
	t.Parallel()

	a := morph.New()
	for _, text := range []string{
		"君の声を探している", "「さよなら」、また明日！", "I'm 2人で…", "ラララ　夜空に光る星よ", "♪", "食べさせられなかった",
	} {
		tokens := analyzeText(t, a, text)
		owner := make([]int, len(tokens))
		for i := range owner {
			owner[i] = -1
		}
		for k, w := range findWords(tokens) {
			if w.FirstToken > w.LastToken {
				t.Errorf("%q: word %d runs backwards", text, k)
			}
			for i := w.FirstToken; i <= w.LastToken; i++ {
				if owner[i] != -1 {
					t.Errorf("%q: token %d is in words %d and %d", text, i, owner[i], k)
				}
				owner[i] = k
			}
		}
		for i, tok := range tokens {
			if inWord := owner[i] != -1; inWord == outsideWords(tok) {
				t.Errorf("%q: token %q in a word = %v, want %v", text, tok.Surface, inWord, !outsideWords(tok))
			}
		}
	}
}

func TestEveryRuleNamesAGrammarPointInTheList(t *testing.T) {
	t.Parallel()

	dict := noMeanings(t)
	ids := []string{
		"te-form", "teiru", "teru", "past", "negative", "nu", "zu", "polite-masu", "continuative", "potential",
		"ra-nuki", "passive", "causative", "volitional", "conditional-ba", "conditional-tara", "imperative",
		"tai-want", "te-shimau", "chau", "adverbial", "na-attributive", "te-iku", "te-kuru", "te-kureru",
		"sou-looks", "darou", "ja-nai", "n-da", "ga-subject", "ga-but", "wa-topic", "wo-object", "ni-place",
		"ni-time", "ni-target", "ni-purpose", "ni-change", "ni-destination", "de-place", "he-direction", "to-when",
		"to-quote", "to-and", "to-with", "tte-quote", "no-nominalizer", "no-question", "no-possession", "te-mo",
		"mo-also", "kara-because", "kara-from", "ka-some", "ka-question", "na-prohibition", "na-feeling",
		"demo-even",
	}
	for _, id := range particleUses {
		ids = append(ids, id)
	}
	for id := range grammarGlosses {
		ids = append(ids, id)
	}
	for id := range stepLabels {
		ids = append(ids, id)
	}
	for _, id := range ids {
		if _, ok := dict.GrammarPoint(id); !ok {
			t.Errorf("grammar point %q is used by the word rules but isn't in grammar.json", id)
		}
	}
}

func TestLemmaReading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tok  Token
		want string
	}{
		{name: "godan stem", tok: Token{Surface: "探し", Reading: "さがし", BaseForm: "探す"}, want: "さがす"},
		{name: "small tsu", tok: Token{Surface: "光っ", Reading: "ひかっ", BaseForm: "光る"}, want: "ひかる"},
		{name: "adjective", tok: Token{Surface: "美しかっ", Reading: "うつくしかっ", BaseForm: "美しい"},
			want: "うつくしい"},
		{name: "suru", tok: Token{Surface: "し", Reading: "し", BaseForm: "する"}, want: "する"},
		{name: "kuru", tok: Token{Surface: "来", Reading: "こ", BaseForm: "来る", ConjType: "カ変・来ル"}, want: "くる"},
		{name: "same as written", tok: Token{Surface: "星", Reading: "ほし", BaseForm: "星"}, want: "ほし"},
		{name: "no reading", tok: Token{Surface: "love"}, want: ""},
		{name: "readings that don't line up", tok: Token{Surface: "行っ", Reading: "ゆき", BaseForm: "行く"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lemmaReading(tt.tok); got != tt.want {
				t.Errorf("lemmaReading(%+v) = %q, want %q", tt.tok, got, tt.want)
			}
		})
	}
}

func TestConjClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		conjType string
		base     string
		want     string
	}{
		{conjType: "五段・サ行", want: "v5s"},
		{conjType: "五段・ラ行", want: "v5r"},
		{conjType: "五段・ラ行", base: "ある", want: "v5r-i"},
		{conjType: "五段・カ行イ音便", want: "v5k"},
		{conjType: "五段・カ行促音便", want: "v5k-s"},
		{conjType: "五段・ワ行促音便", want: "v5u"},
		{conjType: "五段・ワ行ウ音便", want: "v5u-s"},
		{conjType: "五段・タ行", want: "v5t"},
		{conjType: "五段・マ行", want: "v5m"},
		{conjType: "五段・バ行", want: "v5b"},
		{conjType: "五段・ナ行", want: "v5n"},
		{conjType: "五段・ガ行", want: "v5g"},
		{conjType: "五段・ラ行特殊", want: "v5aru"},
		{conjType: "一段", want: "v1"},
		{conjType: "一段・クレル", want: "v1-s"},
		{conjType: "サ変・スル", want: "vs-i"},
		{conjType: "サ変・－スル", want: "vs-s"},
		{conjType: "サ変・－ズル", want: "vz"},
		{conjType: "カ変・来ル", want: "vk"},
		{conjType: "形容詞・アウオ段", want: "adj-i"},
		{conjType: "形容詞・イ段", want: "adj-i"},
		{conjType: "形容詞・イイ", want: "adj-ix"},
		{conjType: "不変化型", want: ""},
		{conjType: "", want: ""},
	}
	for _, tt := range tests {
		if got := conjClass(Token{ConjType: tt.conjType, BaseForm: tt.base}); got != tt.want {
			t.Errorf("conjClass(%s, %s) = %q, want %q", tt.conjType, tt.base, got, tt.want)
		}
	}
}
