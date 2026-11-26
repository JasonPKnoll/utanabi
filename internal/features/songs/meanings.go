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
	words, err := m.joinPhrases(ctx, findWords(line.Tokens), line.Tokens)
	if err != nil {
		return err
	}
	if words, err = m.joinSlangVerbs(ctx, words, line.Tokens); err != nil {
		return err
	}
	if words, err = m.joinSpokenNai(ctx, words, line.Tokens); err != nil {
		return err
	}
	for i := range words {
		w := &words[i]
		if w.Kind != WordContent {
			continue
		}
		if err := m.spokenAdjective(ctx, w, line.Tokens); err != nil {
			return err
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
		if w.EntryID != 0 && numberBeforeCounter(line.Tokens, w.FirstToken) {
			if err := m.readNumber(ctx, w, line.Tokens); err != nil {
				return err
			}
		}
	}
	if err := m.countingRuns(ctx, words, line.Tokens); err != nil {
		return err
	}
	line.Analysis = words
	if line.Analysis == nil {
		line.Analysis = []WordAnalysis{}
	}
	return nil
}

// readNumber reads a number in digits the way the dictionary reads its kanji spelling, since kagome gives digits no
// reading: 1番 is read like 一番, いちばん, so the 1 gets いち and the line's romaji says ichiban. When the reading
// doesn't split at the counter (2人 is ふたり), all of it goes over the number.
func (m *meanings) readNumber(ctx context.Context, w *WordAnalysis, tokens []Token) error {
	entries, err := m.lookup(ctx, w.Lemma, "")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID != w.EntryID {
			continue
		}
		for _, f := range e.Forms {
			if f.Text != w.Lemma || !jtext.IsHiragana(f.Reading) {
				continue
			}
			w.LemmaReading = f.Reading
			number, counter := &tokens[w.FirstToken], &tokens[w.FirstToken+1]
			counterReading := jtext.ToHiragana(counter.Reading)
			if digits, ok := strings.CutSuffix(f.Reading, counterReading); ok && counterReading != "" && digits != "" {
				number.Reading = digits
			} else {
				number.Reading, counter.Reading = f.Reading, ""
			}
			return nil
		}
	}
	return nil
}

// countingRuns reads a katakana word as a number when it stands next to one, as in a count or a chant: サン！ヨン！
// ゼロワン！ is three, four, zero, one, where サン alone is "Sun". It goes over the line until nothing changes, so a
// run of them (サン！サン！ゼロ) turns from the end that touches a number.
func (m *meanings) countingRuns(ctx context.Context, words []WordAnalysis, tokens []Token) error {
	numbers := make([]bool, len(words))
	for k := range words {
		e, err := m.chosenEntry(ctx, words[k])
		if err != nil {
			return err
		}
		numbers[k] = isNumber(e)
	}
	for changed := true; changed; {
		changed = false
		for k := range words {
			w := &words[k]
			if numbers[k] || w.Kind != WordContent || !isKatakana(w.Lemma) || !nextToNumber(words, tokens, numbers, k) {
				continue
			}
			entries, err := m.lookup(ctx, w.Lemma, w.LemmaReading)
			if err != nil {
				return err
			}
			entries = slices.DeleteFunc(slices.Clone(entries), func(e Entry) bool { return !isNumber(e) })
			entry, sense, ok := pick(entries, catNoun, "", w.Lemma)
			if !ok {
				continue
			}
			w.EntryID, w.SenseIndex = entry.ID, sense
			numbers[k], changed = true, true
		}
	}
	return nil
}

// nextToNumber reports whether the word before or after word k is a number, with nothing but punctuation between.
func nextToNumber(words []WordAnalysis, tokens []Token, numbers []bool, k int) bool {
	gap := func(from, to int) bool {
		for _, t := range tokens[from:to] {
			if t.POS != morph.Symbol {
				return false
			}
		}
		return true
	}
	return (k > 0 && numbers[k-1] && gap(words[k-1].LastToken+1, words[k].FirstToken)) ||
		(k+1 < len(words) && numbers[k+1] && gap(words[k].LastToken+1, words[k+1].FirstToken))
}

// isNumber reports whether an entry is a number: a sense marked num, or one whose English is digits (ゼロ, "0").
func isNumber(e Entry) bool {
	for _, s := range e.Senses {
		if slices.Contains(s.PartsOfSpeech, "num") {
			return true
		}
		for _, g := range s.Glosses {
			if g != "" && strings.Trim(g, "0123456789") == "" {
				return true
			}
		}
	}
	return false
}

// chosenEntry gives the dictionary entry the word's meaning was taken from, or an empty one.
func (m *meanings) chosenEntry(ctx context.Context, w WordAnalysis) (Entry, error) {
	if w.EntryID == 0 {
		return Entry{}, nil
	}
	for _, form := range [][2]string{{w.Lemma, w.LemmaReading}, {w.Lemma, ""}, {w.altLemma, w.altReading}, {w.altLemma, ""}} {
		entries, err := m.lookup(ctx, form[0], form[1])
		if err != nil {
			return Entry{}, err
		}
		for _, e := range entries {
			if e.ID == w.EntryID {
				return e, nil
			}
		}
	}
	return Entry{}, nil
}

// joinPhrases makes a set phrase one word when the dictionary has it whole: a noun, then が, を, or に, then a verb,
// as in 気がつく, to notice. Apart, its words would read 気 "spirit" and つく "be lit". The phrase conjugates as its
// verb, so it keeps the verb's steps and class.
func (m *meanings) joinPhrases(ctx context.Context, words []WordAnalysis, tokens []Token) ([]WordAnalysis, error) {
	var out []WordAnalysis
	for k := 0; k < len(words); k++ {
		if k+2 < len(words) && phraseShape(words[k:k+3], tokens) {
			noun, particle, verb := words[k], tokens[words[k+1].FirstToken], words[k+2]
			lemma := noun.Lemma + particle.Surface + verb.Lemma
			entries, err := m.lookup(ctx, lemma, "")
			if err != nil {
				return nil, err
			}
			if len(entries) > 0 {
				out = append(out, phrase(noun, jtext.ToHiragana(particle.Reading), verb, lemma))
				k += 2
				continue
			}
		}
		if k+1 < len(words) && nounThenVerb(words[k], words[k+1], tokens) {
			// 迷惑かける: the spoken phrase drops its particle, so try it back in: 迷惑をかける, cause trouble.
			joined, err := m.withDroppedParticle(ctx, words[k], words[k+1])
			if err != nil {
				return nil, err
			}
			if joined != nil {
				out = append(out, *joined)
				k++
				continue
			}
		}
		out = append(out, words[k])
	}
	return out, nil
}

func (m *meanings) withDroppedParticle(ctx context.Context, noun, verb WordAnalysis) (*WordAnalysis, error) {
	for _, particle := range []string{"を", "が"} {
		lemma := noun.Lemma + particle + verb.Lemma
		entries, err := m.lookup(ctx, lemma, "")
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			w := phrase(noun, particle, verb, lemma)
			return &w, nil
		}
	}
	return nil, nil
}

// phrase makes a noun, a particle, and a verb one word, with the lemma the dictionary has. It conjugates as its verb.
func phrase(noun WordAnalysis, particle string, verb WordAnalysis, lemma string) WordAnalysis {
	reading := ""
	if noun.LemmaReading != "" && verb.LemmaReading != "" {
		reading = noun.LemmaReading + particle + verb.LemmaReading
	}
	verb.FirstToken = noun.FirstToken
	verb.Lemma, verb.LemmaReading = cleanLemma(lemma, reading)
	verb.altLemma, verb.altReading = "", ""
	return verb
}

func phraseShape(w []WordAnalysis, tokens []Token) bool {
	noun, particle, verb := w[0], w[1], w[2]
	if !singleNoun(noun, tokens) {
		return false
	}
	if particle.Kind != WordParticle || particle.FirstToken != noun.LastToken+1 || particle.LastToken != particle.FirstToken {
		return false
	}
	switch tokens[particle.FirstToken].base() {
	case "が", "を", "に":
	default:
		return false
	}
	return verb.Kind == WordContent && verb.FirstToken == particle.LastToken+1 && strings.HasPrefix(verb.ConjClass, "v")
}

// nounThenVerb reports whether a noun is followed straight away by a verb, with no particle between them.
func nounThenVerb(noun, verb WordAnalysis, tokens []Token) bool {
	return singleNoun(noun, tokens) && verb.Kind == WordContent && verb.FirstToken == noun.LastToken+1 &&
		strings.HasPrefix(verb.ConjClass, "v")
}

func singleNoun(w WordAnalysis, tokens []Token) bool {
	return w.Kind == WordContent && w.FirstToken == w.LastToken && tokens[w.FirstToken].POS == morph.Noun
}

// category is the broad kind of a content word, for matching it to a dictionary sense.
type category int

const (
	catUnknown category = iota
	catNoun
	// catAdverbialNoun is a noun kagome says can be used as an adverb, such as いつか or 今日, used as one: no particle
	// or copula after it.
	catAdverbialNoun
	catPronoun
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
		case morph.Pronoun:
			return catPronoun
		case morph.Noun, morph.Suffix:
			if tokens[i].Detail == "副詞可能" && w.FirstToken == w.LastToken && usedAsAdverb(tokens, w.LastToken+1) {
				return catAdverbialNoun
			}
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

// usedAsAdverb reports whether a word that ends before token from is used as an adverb: the line goes on, past any
// spaces, with a word that isn't a particle or the copula. いつかきっと is "someday"; 全てが is "everything".
func usedAsAdverb(tokens []Token, from int) bool {
	for _, t := range tokens[from:] {
		switch t.POS {
		case morph.Symbol:
			continue
		case morph.Particle, morph.Auxiliary:
			return false
		}
		return true
	}
	return false
}

// senseFits reports whether a sense with the given JMdict part of speech codes can be the word's meaning.
func senseFits(cat category, codes []string) bool {
	return slices.ContainsFunc(codes, func(c string) bool { return codeFits(cat, c) })
}

func codeFits(cat category, c string) bool {
	switch cat {
	case catNoun:
		return c == "n" || c == "pn" || c == "num" || c == "ctr" || strings.HasPrefix(c, "n-")
	case catAdverbialNoun:
		return codeFits(catNoun, c) || codeFits(catAdverb, c)
	case catPronoun:
		return c == "pn"
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
		entry, sense, ok := pick(entries, cat, w.ConjClass, try[0])
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

// pick chooses the entry that best fits the word, and the first of its senses that fits, or sense 0. The tests, each
// outweighing all the ones after it:
//   - a sense that fits the word's part of speech (for a pronoun, a pronoun sense, then a noun one: なん is 何, not 難);
//   - a sense that names the word's conjugation class;
//   - a spelling exactly as the line writes it (ホラ is 法螺 "boasting", spelled ホラ, not 洞 "hollow", spelled ほら);
//   - a spelling of the word in common use (だれ is 誰, whose だれ is common, not 垂れ, whose common spelling is たれ);
//   - for a word written in kana, a sense usually written in kana (いる is 居る, not 射る).
//
// Ties go to the dictionary's order, most common first.
func pick(entries []Entry, cat category, class, form string) (Entry, int, bool) {
	kana := jtext.IsKana(form)
	reading := ""
	if kana {
		reading = jtext.ToHiragana(form)
	}
	spelled := func(f Form) bool { return f.Text == form || (kana && f.Reading == reading) }
	best, bestSense := -1, 0
	var bestScore [5]int
	for i, e := range entries {
		if len(e.Senses) == 0 {
			continue
		}
		var score [5]int
		fit, sense := senseFit(e.Senses, cat)
		score[0] = fit
		if class != "" && slices.ContainsFunc(e.Senses, func(s Sense) bool { return slices.Contains(s.PartsOfSpeech, class) }) {
			score[1] = 1
		}
		if slices.ContainsFunc(e.Forms, func(f Form) bool { return f.Text == form }) {
			score[2] = 1
		}
		if slices.ContainsFunc(e.Forms, func(f Form) bool { return f.Common && spelled(f) }) {
			score[3] = 1
		}
		if kana && slices.ContainsFunc(e.Senses, func(s Sense) bool { return slices.Contains(s.Misc, "uk") }) {
			score[4] = 1
		}
		if best < 0 || slices.Compare(score[:], bestScore[:]) > 0 {
			best, bestScore, bestSense = i, score, sense
		}
	}
	if best < 0 {
		return Entry{}, 0, false
	}
	return entries[best], bestSense, true
}

// senseFit scores how well an entry's senses fit the word's part of speech: 2 for a sense that fits, 1 for a pronoun
// with only a noun sense or an adverbial noun with no adverb sense, 0 for none. It returns the first sense at that score, or the first sense.
func senseFit(senses []Sense, cat category) (int, int) {
	if cat == catAdverbialNoun {
		// いつか is "someday", an adverb, before 五日, the fifth of the month.
		for _, s := range senses {
			if senseFits(catAdverb, s.PartsOfSpeech) {
				return 2, s.Index
			}
		}
		cat = catNoun
		for _, s := range senses {
			if senseFits(cat, s.PartsOfSpeech) {
				return 1, s.Index
			}
		}
		return 0, senses[0].Index
	}
	for _, s := range senses {
		if senseFits(cat, s.PartsOfSpeech) {
			return 2, s.Index
		}
	}
	if cat == catPronoun {
		for _, s := range senses {
			if senseFits(catNoun, s.PartsOfSpeech) {
				return 1, s.Index
			}
		}
	}
	return 0, senses[0].Index
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
				case strings.HasPrefix(c, "v5") && (slices.Equal(w.Steps, []string{"continuative"}) ||
					slices.Equal(w.Steps, []string{"imperative"})):
					// がんばれ: a potential verb's bare stem or command is the godan verb's command, which is spelled
					// the same. A potential is hardly ever told to do something.
					w.Lemma, w.LemmaReading = candidate, formReading(e, candidate, candidateReading)
					w.ConjClass = c
					w.Steps = []string{"imperative"}
					return nil
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
