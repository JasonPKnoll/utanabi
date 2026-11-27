package songs

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// slangEndings are the endings a verb made from a katakana word takes, as kagome splits them off the word (オナ, っ,
// てる), with the conjugation steps each one is.
var slangEndings = map[string][]string{
	"る": nil, "った": {"past"}, "って": {"te-form"}, "ってる": {"te-form", "teru"}, "ってた": {"te-form", "teru", "past"},
	"っている": {"te-form", "teiru"}, "らない": {"negative"}, "らねえ": {"negative"},
}

// joinSlangVerbs makes a katakana word and the verb ending after it one verb, when the dictionary has the word with る
// as a godan verb: オナってる is オナる in the 〜てる form, where kagome reads オナ, then って "(quote)" and る. The
// ending's tokens are marked as conjugation pieces, so the romaji writes the verb as one word.
func (m *meanings) joinSlangVerbs(ctx context.Context, words []WordAnalysis, tokens []Token) ([]WordAnalysis, error) {
	var out []WordAnalysis
	for k := 0; k < len(words); k++ {
		w := words[k]
		if !singleNoun(w, tokens) || !isKatakana(tokens[w.FirstToken].Surface) {
			out = append(out, w)
			continue
		}
		end, steps := slangEnding(words[k+1:], tokens, w.LastToken+1)
		if end < 0 {
			out = append(out, w)
			continue
		}
		stem := tokens[w.FirstToken].Surface
		lemma, reading := stem+"る", jtext.ToHiragana(stem)+"る"
		entries, err := m.lookup(ctx, lemma, reading)
		if err != nil {
			return nil, err
		}
		if !hasCode(entries, func(c string) bool { return c == "v5r" }) {
			out = append(out, w)
			continue
		}
		verb := WordAnalysis{FirstToken: w.FirstToken, LastToken: words[k+end].LastToken, Kind: WordContent,
			ConjClass: "v5r", Steps: steps, Source: AnalysisAuto, head: w.FirstToken}
		verb.Lemma, verb.LemmaReading = cleanLemma(lemma, reading)
		tokens[w.FirstToken].POS = morph.Verb
		for i := w.LastToken + 1; i <= verb.LastToken; i++ {
			tokens[i].POS = morph.Auxiliary
		}
		out = append(out, verb)
		k += end
	}
	return out, nil
}

// slangEnding finds the longest run of the words after a katakana word, from token from, that spells one of the
// slangEndings. It gives the number of words in it, or -1, and the ending's steps.
func slangEnding(after []WordAnalysis, tokens []Token, from int) (int, []string) {
	found, steps := -1, []string(nil)
	var text strings.Builder
	for n, w := range after {
		if w.FirstToken != from {
			break
		}
		for i := w.FirstToken; i <= w.LastToken; i++ {
			text.WriteString(tokens[i].Surface)
		}
		if s, ok := slangEndings[text.String()]; ok {
			found, steps = n+1, s
		}
		from = w.LastToken + 1
	}
	return found, steps
}

func isKatakana(s string) bool {
	return s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !unicode.In(r, unicode.Katakana) && r != 'ー' })
}

// spokenNaiEnds are the spoken ねえ for ない at the end of a word, with a sentence-final particle sung onto it.
var spokenNaiEnds = []string{"ねえ", "ねぇ", "ねー"}

var finalParticles = []string{"", "よ", "ね", "な", "ぞ", "ぜ", "わ"}

// joinSpokenNai makes one word of the pieces of an adjective that ends in ない, said with ねえ, when the dictionary
// has the adjective: はんぱねえ is 半端ない, "incredible", where kagome reads はん "seal" and ぱねえ, and 仕方ねえ is
// 仕方ない, where it reads ねえ as a grammar word. It tries two words together, then one, so a word kagome kept whole
// (ぱねえ) only changes when its pieces make a longer word.
func (m *meanings) joinSpokenNai(ctx context.Context, words []WordAnalysis, tokens []Token) ([]WordAnalysis, error) {
	var out []WordAnalysis
	for k := 0; k < len(words); k++ {
		joined := false
		for n := 2; n >= 1 && !joined; n-- {
			if k+n > len(words) {
				continue
			}
			w, ok, err := m.spokenNai(ctx, words[k:k+n], tokens)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, w)
				k += n - 1
				joined = true
			}
		}
		if !joined {
			out = append(out, words[k])
		}
	}
	return out, nil
}

func (m *meanings) spokenNai(ctx context.Context, words []WordAnalysis, tokens []Token) (WordAnalysis, bool, error) {
	var text, reading strings.Builder
	for i, w := range words {
		if i > 0 && w.FirstToken != words[i-1].LastToken+1 {
			return WordAnalysis{}, false, nil
		}
		for j := w.FirstToken; j <= w.LastToken; j++ {
			t := tokens[j]
			spokenNe := t.POS == morph.Adjective && slices.Contains(spokenNaiEnds, t.Surface)
			if (t.POS == morph.Verb || t.POS == morph.Adjective || t.POS == morph.Symbol) && !spokenNe {
				// 知らねえ is a verb's own negative, not an adjective.
				return WordAnalysis{}, false, nil
			}
			text.WriteString(t.Surface)
			reading.WriteString(t.Reading)
		}
	}
	stem, stemReading, ok := cutSpokenNai(text.String(), jtext.ToHiragana(reading.String()))
	if !ok || stem == "" {
		return WordAnalysis{}, false, nil
	}
	entries, err := m.lookup(ctx, stem+"ない", stemReading+"ない")
	if err != nil || !hasCode(entries, func(c string) bool { return c == "adj-i" }) {
		return WordAnalysis{}, false, err
	}
	w := WordAnalysis{FirstToken: words[0].FirstToken, LastToken: words[len(words)-1].LastToken, Kind: WordContent,
		ConjClass: "adj-i", Source: AnalysisAuto, head: words[0].FirstToken}
	w.Lemma, w.LemmaReading = cleanLemma(stem+"ない", stemReading+"ない")
	return w, true, nil
}

// cutSpokenNai splits はんぱねえよ into はんぱ, the adjective before its ねえ, and gives its reading too.
func cutSpokenNai(text, reading string) (string, string, bool) {
	for _, nee := range spokenNaiEnds {
		for _, p := range finalParticles {
			stem, ok := strings.CutSuffix(text, nee+p)
			stemReading, rok := strings.CutSuffix(reading, jtext.ToHiragana(nee)+p)
			if ok && rok {
				return stem, stemReading, true
			}
		}
	}
	return "", "", false
}

// eRow is the kana an adjective's last sound before い becomes when it is said roughly: くさい is くせえ, すごい すげえ.
var eRow = map[rune]rune{
	'あ': 'え', 'お': 'え', 'か': 'け', 'こ': 'け', 'が': 'げ', 'ご': 'げ', 'さ': 'せ', 'そ': 'せ', 'ざ': 'ぜ', 'ぞ': 'ぜ',
	'た': 'て', 'と': 'て', 'だ': 'で', 'ど': 'で', 'な': 'ね', 'の': 'ね', 'は': 'へ', 'ほ': 'へ', 'ば': 'べ', 'ぼ': 'べ',
	'ぱ': 'ぺ', 'ぽ': 'ぺ', 'ま': 'め', 'も': 'め', 'ら': 'れ', 'ろ': 'れ', 'わ': 'え',
}

// spokenAdjective spots an adjective said roughly, its あい or おい turned into ええ, that kagome read as a command:
// 臭え is 臭い, くせえ, "stinks", not 臭う "be fragrant!". It needs the verb to be a godan う verb in its え command
// form, and the dictionary to have the kanji with い as an adjective whose reading turns into the spoken one. The
// token's reading is corrected, so the furigana and romaji say くせえ.
func (m *meanings) spokenAdjective(ctx context.Context, w *WordAnalysis, tokens []Token) error {
	if w.FirstToken != w.LastToken {
		return nil
	}
	t := &tokens[w.FirstToken]
	stem, ok := strings.CutSuffix(t.Surface, "え")
	if !ok || t.POS != morph.Verb || !strings.HasPrefix(t.ConjForm, "命令") || !strings.HasPrefix(t.ConjType, "五段・ワ行") ||
		!strings.ContainsFunc(stem, func(r rune) bool { return unicode.In(r, unicode.Han) }) {
		return nil
	}
	entries, err := m.lookup(ctx, stem+"い", "")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !hasCode([]Entry{e}, func(c string) bool { return c == "adj-i" }) {
			continue
		}
		for _, f := range e.Forms {
			r := []rune(f.Reading)
			n := len(r)
			if f.Text != stem+"い" || n < 2 || r[n-1] != 'い' {
				continue
			}
			row, ok := eRow[r[n-2]]
			if !ok {
				continue
			}
			t.Reading = string(r[:n-2]) + string(row) + "え"
			w.Lemma, w.LemmaReading = cleanLemma(f.Text, f.Reading)
			w.ConjClass = "adj-i"
			w.Steps = slices.DeleteFunc(w.Steps, func(s string) bool { return s == "imperative" })
			return nil
		}
	}
	return nil
}
