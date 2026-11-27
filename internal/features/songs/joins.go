package songs

import (
	"context"
	"slices"
	"strings"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// joinCompounds makes a verb's stem and the word after it one noun when the dictionary has them together as a noun:
// 分け隔て, "distinction", where kagome reads 分け "divide" and 隔て "partition". A noun made from a verb's stem is a
// common way Japanese builds words (書き手, 生まれ変わり). A の just before such a noun is "'s", not the subject of
// a verb, as it looked while the stem stood alone.
func (m *meanings) joinCompounds(ctx context.Context, words []WordAnalysis, tokens []Token) ([]WordAnalysis, error) {
	var out []WordAnalysis
	for k := 0; k < len(words); k++ {
		w := words[k]
		if k+1 >= len(words) || !stemAlone(w, tokens) {
			out = append(out, w)
			continue
		}
		next := words[k+1]
		if next.Kind != WordContent || next.FirstToken != w.LastToken+1 || next.FirstToken != next.LastToken {
			out = append(out, w)
			continue
		}
		t := tokens[next.FirstToken]
		if t.POS != morph.Noun && !stemAlone(next, tokens) {
			out = append(out, w)
			continue
		}
		surface := tokens[w.FirstToken].Surface + t.Surface
		entries, err := m.lookup(ctx, surface, "")
		if err != nil {
			return nil, err
		}
		if !hasCode(entries, func(c string) bool { return c == "n" }) {
			out = append(out, w)
			continue
		}
		joined := WordAnalysis{FirstToken: w.FirstToken, LastToken: next.LastToken, Kind: WordContent,
			Source: AnalysisAuto, head: w.FirstToken}
		joined.Lemma, joined.LemmaReading = cleanLemma(surface,
			jtext.ToHiragana(tokens[w.FirstToken].Reading+t.Reading))
		if n := len(out); n > 0 && out[n-1].Kind == WordParticle && out[n-1].GrammarPointID == "no-subject" {
			out[n-1].GrammarPointID = "no-possession"
		}
		out = append(out, joined)
		k++
	}
	return out, nil
}

// stemAlone reports whether w is a verb's bare stem and nothing more: 分け in 分け隔て.
func stemAlone(w WordAnalysis, tokens []Token) bool {
	t := tokens[w.FirstToken]
	return w.Kind == WordContent && w.FirstToken == w.LastToken && t.POS == morph.Verb && t.ConjForm == "連用形"
}

// teAdverbs makes a verb's te form an adverb when the dictionary has it as one and another verb follows: まとめて
// かかってきなさい is "come at me all at once", not "collect"; 揃って歩く is "walk all together".
func (m *meanings) teAdverbs(ctx context.Context, words []WordAnalysis, tokens []Token) error {
	for k := 0; k+1 < len(words); k++ {
		w, next := &words[k], words[k+1]
		if w.Kind != WordContent || w.LastToken != w.FirstToken+1 || next.Kind != WordContent ||
			tokens[next.head].POS != morph.Verb {
			continue
		}
		verb, te := tokens[w.FirstToken], tokens[w.LastToken]
		if verb.POS != morph.Verb || te.POS != morph.Particle || (te.base() != "て" && te.base() != "で") {
			continue
		}
		surface := verb.Surface + te.Surface
		entries, err := m.lookup(ctx, surface, "")
		if err != nil {
			return err
		}
		if !hasCode(entries, func(c string) bool { return c == "adv" || c == "exp" }) {
			continue
		}
		w.Lemma, w.LemmaReading = cleanLemma(surface, jtext.ToHiragana(verb.Reading+te.Reading))
		w.ConjClass, w.Steps = "", nil
	}
	return nil
}

// joinUnknown joins a piece kagome doesn't know (it gives no dictionary form, as for ゃない) with the words around
// it, when the dictionary has them together: 知ったこっちゃない, "none of my business", which kagome cuts into
// 知った, こっち "this way", and ゃない. The longest join the dictionary has wins.
func (m *meanings) joinUnknown(ctx context.Context, words []WordAnalysis, tokens []Token) ([]WordAnalysis, error) {
	for k := 0; k < len(words); k++ {
		if !unknownPiece(words[k], tokens) {
			continue
		}
		joined, from, to, err := m.longestJoin(ctx, words, tokens, k)
		if err != nil {
			return nil, err
		}
		if from < 0 {
			continue
		}
		words = slices.Replace(words, from, to+1, joined)
		k = from
	}
	return words, nil
}

func unknownPiece(w WordAnalysis, tokens []Token) bool {
	t := tokens[w.FirstToken]
	return w.Kind == WordContent && w.FirstToken == w.LastToken && t.BaseForm == "" && hasJapanese(t.Surface)
}

// longestJoin tries the words from up to three before word k to one after it, longest first, and returns the first
// run the dictionary has, joined, with where it starts and ends; from is -1 when none is.
func (m *meanings) longestJoin(ctx context.Context, words []WordAnalysis, tokens []Token, k int) (
	WordAnalysis, int, int, error,
) {
	type span struct{ from, to int }
	var spans []span
	for from := max(0, k-3); from <= k; from++ {
		for to := min(len(words)-1, k+1); to >= k; to-- {
			if to > from && contiguous(words[from:to+1]) {
				spans = append(spans, span{from, to})
			}
		}
	}
	slices.SortStableFunc(spans, func(a, b span) int { return (b.to - b.from) - (a.to - a.from) })
	for _, s := range spans {
		first, last := words[s.from].FirstToken, words[s.to].LastToken
		var text, reading strings.Builder
		for _, t := range tokens[first : last+1] {
			text.WriteString(t.Surface)
			reading.WriteString(t.Reading)
		}
		entries, err := m.lookup(ctx, text.String(), "")
		if err != nil {
			return WordAnalysis{}, -1, -1, err
		}
		if len(entries) == 0 {
			continue
		}
		w := WordAnalysis{FirstToken: first, LastToken: last, Kind: WordContent, Source: AnalysisAuto, head: first}
		w.Lemma, w.LemmaReading = cleanLemma(text.String(), jtext.ToHiragana(reading.String()))
		return w, s.from, s.to, nil
	}
	return WordAnalysis{}, -1, -1, nil
}

func contiguous(words []WordAnalysis) bool {
	for i := 1; i < len(words); i++ {
		if words[i].FirstToken != words[i-1].LastToken+1 {
			return false
		}
	}
	return true
}

// spokenKotoAt reports whether こた at i, after a verb, is ことは said quickly: 肩落とすこたない, there's no need to hang
// your head. kagome reads it as 濃い "deep" in the past, and takes a ない after it into the same word.
func spokenKotoAt(tokens []Token, i int) bool {
	if i == 0 || i+1 >= len(tokens) {
		return false
	}
	ko, ta, prev := tokens[i], tokens[i+1], tokens[i-1]
	return ko.Surface == "こ" && ko.base() == "こい" && ta.Surface == "た" && prev.POS == morph.Verb &&
		prev.ConjForm == "基本形"
}

// spokenKoto makes こた at i the word こと, with た as its particle, and nothing after it in the word.
func spokenKoto(tokens []Token, i int) WordAnalysis {
	tokens[i].POS, tokens[i+1].POS = morph.Noun, morph.Particle
	return WordAnalysis{FirstToken: i, LastToken: i + 1, Kind: WordContent, Source: AnalysisAuto, head: i,
		Lemma: "こと", LemmaReading: "こと"}
}

// formalNouns are the nouns that grammar uses for their meaning more than their sense, written in kana, with the
// kanji that spells that meaning: わけ is 訳, "reason; that's why", not 分け, "division", which sounds the same.
var formalNouns = map[string]string{
	"わけ": "訳", "はず": "筈", "つもり": "積もり", "ため": "為", "ほう": "方", "ところ": "所", "とこ": "所",
}

// formalNoun gives a formal noun (kagome's 非自立 noun) the kanji of its meaning to look up first.
func formalNoun(w *WordAnalysis, tokens []Token) {
	t := tokens[w.head]
	kanji, ok := formalNouns[t.Surface]
	if !ok || w.FirstToken != w.LastToken || t.POS != morph.Noun || t.Detail != "非自立" {
		return
	}
	w.altLemma, w.altReading = w.Lemma, w.LemmaReading
	w.Lemma, w.LemmaReading = kanji, jtext.ToHiragana(t.Surface)
}

// comeCommand gives a command made with 〜てくる (かかってきなさい, かかってこい) the meaning the dictionary has for
// its 〜てこい form, when it has one: かかってこい is "bring it on!", where かかる alone is first "to take (time)".
func (m *meanings) comeCommand(ctx context.Context, w *WordAnalysis, tokens []Token) error {
	n := len(w.Steps)
	if n < 2 || w.Steps[n-1] != "imperative" || !slices.Contains(w.Steps, "te-kuru") {
		return nil
	}
	var prefix strings.Builder
	for _, t := range tokens[w.FirstToken : w.LastToken+1] {
		if t.base() == "くる" || t.base() == "来る" {
			entries, err := m.lookup(ctx, prefix.String()+"こい", "")
			if err != nil || len(entries) == 0 {
				return err
			}
			w.EntryID, w.SenseIndex = entries[0].ID, 0
			return nil
		}
		prefix.WriteString(t.Surface)
	}
	return nil
}

// negativeAdjective reads a verb's spoken negative as the adjective the dictionary has for it, when it has one:
// つまらねぇ is つまらない, "boring", not つまる "be packed" made negative; くだらねぇ is "worthless". kagome reads the
// plain つまらない as the adjective already; a plain negative such as 変わらない stays the verb's, which says more.
func (m *meanings) negativeAdjective(ctx context.Context, w *WordAnalysis, tokens []Token) error {
	first, last := tokens[w.FirstToken], tokens[w.LastToken]
	if w.Kind != WordContent || len(w.Steps) != 1 || w.Steps[0] != "negative" || first.POS != morph.Verb ||
		first.ConjForm != "未然形" || !slices.Contains(spokenNaiEnds, last.Surface) {
		return nil
	}
	form := first.Surface + "ない"
	entries, err := m.lookup(ctx, form, "")
	if err != nil || !hasCode(entries, func(c string) bool { return c == "adj-i" }) {
		return err
	}
	w.Lemma, w.LemmaReading = cleanLemma(form, jtext.ToHiragana(first.Reading)+"ない")
	w.ConjClass, w.Steps = "adj-i", nil
	tokens[w.FirstToken].POS = morph.Adjective
	return nil
}
