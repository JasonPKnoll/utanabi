package songs

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// maxLookupLen is the longest form or reading the dictionary looks up.
const maxLookupLen = 100

// meanings looks words up in the dictionary for one call, and remembers each answer, since a song repeats its words.
type meanings struct {
	dict  Dictionary
	cache map[[2]string][]Entry
	// missing is set once the dictionary says it isn't installed, so no more lookups are tried.
	missing bool
}

func newMeanings(dict Dictionary) *meanings {
	return &meanings{dict: dict, cache: map[[2]string][]Entry{}}
}

func (m *meanings) lookup(ctx context.Context, form, reading string) ([]Entry, error) {
	if m.missing || form == "" || utf8.RuneCountInString(form) > maxLookupLen ||
		utf8.RuneCountInString(reading) > maxLookupLen {
		return nil, nil
	}
	key := [2]string{form, reading}
	if entries, ok := m.cache[key]; ok {
		return entries, nil
	}
	entries, err := m.dict.Lookup(ctx, form, reading)
	if errors.Is(err, ErrNoMeanings) {
		m.missing = true
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.cache[key] = entries
	return entries, nil
}

// analyzeWords sets a line's words from its tokens, and looks up the meaning of each content word.
func (m *meanings) analyzeWords(ctx context.Context, line *Line) error {
	words := findWords(line.Tokens)
	for i := range words {
		w := &words[i]
		if w.Kind != WordContent {
			continue
		}
		cat := wordCategory(*w, line.Tokens)
		if cat == catVerb && w.ConjClass == "v1" {
			if err := m.potential(ctx, w); err != nil {
				return err
			}
		}
		if _, err := m.resolve(ctx, w, cat); err != nil {
			return err
		}
	}
	line.Analysis = words
	if line.Analysis == nil {
		line.Analysis = []WordAnalysis{}
	}
	return nil
}

// category is the broad kind of a content word, for matching it to a dictionary sense.
type category int

const (
	catUnknown category = iota
	catNoun
	catVerb
	catAdjective
	catNaAdjective
	catAdverb
	catConjunction
	catInterjection
	catPrenominal
	catPrefix
)

func wordCategory(w WordAnalysis, tokens []Token) category {
	switch {
	case w.ConjClass == "adj-na":
		return catNaAdjective
	case strings.HasPrefix(w.ConjClass, "adj"):
		return catAdjective
	case strings.HasPrefix(w.ConjClass, "v"):
		return catVerb
	}
	for i := w.FirstToken; i <= w.LastToken && i < len(tokens); i++ {
		switch tokens[i].POS {
		case morph.Prefix:
			if i == w.LastToken {
				return catPrefix
			}
		case morph.Noun, morph.Pronoun, morph.Suffix:
			return catNoun
		case morph.Verb:
			return catVerb
		case morph.Adjective:
			return catAdjective
		case morph.Adverb:
			return catAdverb
		case morph.Conjunction:
			return catConjunction
		case morph.Interjection:
			return catInterjection
		case morph.Other:
			return catPrenominal
		default:
			return catUnknown
		}
	}
	return catUnknown
}

// senseFits reports whether a sense with the given JMdict part of speech codes can be the word's meaning.
func senseFits(cat category, codes []string) bool {
	return slices.ContainsFunc(codes, func(c string) bool { return codeFits(cat, c) })
}

func codeFits(cat category, c string) bool {
	switch cat {
	case catNoun:
		return c == "n" || c == "pn" || c == "num" || c == "ctr" || strings.HasPrefix(c, "n-")
	case catVerb:
		return isVerbCode(c)
	case catAdjective:
		return c == "adj-i" || c == "adj-ix"
	case catNaAdjective:
		return c == "adj-na"
	case catAdverb:
		return c == "adv" || c == "adv-to"
	case catConjunction:
		return c == "conj"
	case catInterjection:
		return c == "int"
	case catPrenominal:
		return c == "adj-pn" || c == "adj-f"
	case catPrefix:
		return c == "pref"
	}
	return true
}

// isVerbCode reports whether c is a JMdict verb class. vi and vt only say whether the verb takes an object.
func isVerbCode(c string) bool {
	return strings.HasPrefix(c, "v") && c != "vi" && c != "vt"
}

// conjClasses are the JMdict codes line_words.conj_class accepts.
var conjClasses = []string{
	"adj-i", "adj-ix", "adj-na", "adj-ku", "adj-shiku", "adj-nari", "v1", "v1-s", "v5aru", "v5b", "v5g", "v5k",
	"v5k-s", "v5m", "v5n", "v5r", "v5r-i", "v5s", "v5t", "v5u", "v5u-s", "v5uru", "vk", "vn", "vr", "vs", "vs-c",
	"vs-i", "vs-s", "vz",
}

// resolve picks the dictionary entry and sense of a content word, and takes the word's conjugation class from the
// dictionary when it has one. It tries the lemma with its reading, then without, then the shorter form. It reports
// whether it found an entry.
func (m *meanings) resolve(ctx context.Context, w *WordAnalysis, cat category) (bool, error) {
	var tries [][2]string
	for _, form := range [][2]string{{w.Lemma, w.LemmaReading}, {w.altLemma, w.altReading}} {
		if form[0] == "" {
			continue
		}
		tries = append(tries, form)
		if form[1] != "" {
			tries = append(tries, [2]string{form[0], ""})
		}
	}
	for _, try := range tries {
		entries, err := m.lookup(ctx, try[0], try[1])
		if err != nil {
			return false, err
		}
		if len(entries) == 0 {
			continue
		}
		entry, sense, ok := pick(entries, cat, w.ConjClass, jtext.IsKana(try[0]))
		if !ok {
			continue
		}
		w.EntryID, w.SenseIndex = entry.ID, sense
		if class := dictionaryClass(entry, sense, cat, w.ConjClass); class != "" {
			w.ConjClass = class
		}
		return true, nil
	}
	return false, nil
}

// pick chooses the entry that best fits the word, and the first of its senses that fits, or sense 0. An entry with a
// fitting sense wins, then one whose senses name the word's conjugation class, then, for a word written in kana, one
// usually written in kana (いる is 居る, not 射る). Ties go to the dictionary's order, most common first.
func pick(entries []Entry, cat category, class string, kana bool) (Entry, int, bool) {
	best, bestScore, bestSense := -1, -1, 0
	for i, e := range entries {
		if len(e.Senses) == 0 {
			continue
		}
		score, sense := 0, e.Senses[0].Index
		for _, s := range e.Senses {
			if senseFits(cat, s.PartsOfSpeech) {
				score, sense = 4, s.Index
				break
			}
		}
		if class != "" && slices.ContainsFunc(e.Senses, func(s Sense) bool { return slices.Contains(s.PartsOfSpeech, class) }) {
			score += 2
		}
		if kana && slices.ContainsFunc(e.Senses, func(s Sense) bool { return slices.Contains(s.Misc, "uk") }) {
			score++
		}
		if score > bestScore {
			best, bestScore, bestSense = i, score, sense
		}
	}
	if best < 0 {
		return Entry{}, 0, false
	}
	return entries[best], bestSense, true
}

// dictionaryClass is the conjugation class the dictionary gives a verb or adjective: the first fitting code of the
// chosen sense, then of the entry's other senses. A noun used with する keeps vs.
func dictionaryClass(e Entry, sense int, cat category, current string) string {
	if cat != catVerb && cat != catAdjective && cat != catNaAdjective {
		return ""
	}
	if current == "vs" {
		return ""
	}
	senses := slices.Clone(e.Senses)
	slices.SortStableFunc(senses, func(a, b Sense) int {
		switch {
		case a.Index == sense:
			return -1
		case b.Index == sense:
			return 1
		}
		return 0
	})
	for _, s := range senses {
		for _, c := range s.PartsOfSpeech {
			if c != "vs" && codeFits(cat, c) && slices.Contains(conjClasses, c) {
				return c
			}
		}
	}
	return ""
}

// eRowToURow turns the kana before る in a potential verb back into the godan ending: 会える is 会う, 書ける is 書く.
var eRowToURow = map[rune]rune{
	'え': 'う', 'け': 'く', 'げ': 'ぐ', 'せ': 'す', 'て': 'つ', 'ね': 'ぬ', 'べ': 'ぶ', 'め': 'む', 'れ': 'る',
}

// potential spots a potential verb that the analyzer read as a verb of its own. kagome reads 会えない as 会える, an
// ichidan verb, and ない. When the dictionary doesn't have 会える as a verb but has 会う as a godan verb, the word is 会う
// in the potential form. In the same way, 見れる is 見る without its ら (ら抜き), when the dictionary has 見る as an ichidan
// or kuru verb. It needs the dictionary: without it the word stays as kagome read it.
func (m *meanings) potential(ctx context.Context, w *WordAnalysis) error {
	lemma, reading := []rune(w.Lemma), []rune(w.LemmaReading)
	n := len(lemma)
	if n < 2 || lemma[n-1] != 'る' {
		return nil
	}
	u, ok := eRowToURow[lemma[n-2]]
	if !ok {
		return nil
	}
	own, err := m.lookup(ctx, w.Lemma, w.LemmaReading)
	if err != nil || hasCode(own, func(c string) bool { return c == "v1" }) {
		return err
	}

	candidate := string(lemma[:n-2]) + string(u)
	var candidateReading string
	if r := len(reading); r >= 2 && reading[r-1] == 'る' && eRowToURow[reading[r-2]] == u {
		candidateReading = string(reading[:r-2]) + string(u)
	}
	entries, err := m.lookup(ctx, candidate, candidateReading)
	if err == nil && len(entries) == 0 && candidateReading != "" {
		entries, err = m.lookup(ctx, candidate, "")
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		for _, s := range e.Senses {
			for _, c := range s.PartsOfSpeech {
				var step string
				switch {
				case strings.HasPrefix(c, "v5"):
					step = "potential"
				case u == 'る' && (c == "v1" || c == "vk"):
					step = "ra-nuki"
				default:
					continue
				}
				w.Lemma, w.LemmaReading = candidate, formReading(e, candidate, candidateReading)
				w.ConjClass = c
				w.Steps = append([]string{step}, w.Steps...)
				return nil
			}
		}
	}
	return nil
}

func hasCode(entries []Entry, match func(string) bool) bool {
	for _, e := range entries {
		for _, s := range e.Senses {
			if slices.ContainsFunc(s.PartsOfSpeech, match) {
				return true
			}
		}
	}
	return false
}

// formReading is the entry's reading of the spelling text: くる for 来る, where the analyzer read 来れる as これる.
func formReading(e Entry, text, fallback string) string {
	for _, f := range e.Forms {
		if f.Text == text && (fallback == "" || f.Reading == fallback) {
			return f.Reading
		}
	}
	for _, f := range e.Forms {
		if f.Text == text {
			return f.Reading
		}
	}
	return fallback
}
