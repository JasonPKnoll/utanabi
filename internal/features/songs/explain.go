package songs

import (
	"context"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

const maxGlossLen = 60

// present works out what the song view shows for each line from its stored analysis: furigana, romaji, the words
// with their meanings and grammar, and the word-by-word gloss.
func present(ctx context.Context, dict Dictionary, song Song) (Song, error) {
	entries, err := wordEntries(ctx, dict, song.Lines)
	if err != nil {
		return Song{}, err
	}
	lines := make([]Line, len(song.Lines))
	for i, line := range song.Lines {
		line.Ruby, line.Romaji = ruby(line), romaji(line.Tokens)
		line.Words, line.Pieces, line.Gloss = explainLine(dict, entries, line)
		lines[i] = line
	}
	song.Lines = lines
	return song, nil
}

// wordEntries reads the dictionary entries the song's words point at, by ID. Without the dictionary there are none.
func wordEntries(ctx context.Context, dict Dictionary, lines []Line) (map[int64]Entry, error) {
	var ids []int64
	for _, line := range lines {
		for _, w := range line.Analysis {
			if w.EntryID != 0 && !slices.Contains(ids, w.EntryID) {
				ids = append(ids, w.EntryID)
			}
		}
	}
	found := map[int64]Entry{}
	if len(ids) == 0 {
		return found, nil
	}
	list, err := dict.Entries(ctx, ids)
	if errors.Is(err, ErrNoMeanings) {
		return found, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		found[e.ID] = e
	}
	return found, nil
}

// explainLine builds a line's words, its pieces, and its gloss. Words that don't fit the line's tokens, which can
// only happen when a song changes while it is read, are left out.
func explainLine(dict Dictionary, entries map[int64]Entry, line Line) ([]Word, []LinePiece, string) {
	words := []Word{}
	pieces := []LinePiece{}
	if len(line.Tokens) == 0 {
		return words, append(pieces, LinePiece{Ruby: []RubyPart{{Text: line.Text}}, Word: -1}), ""
	}
	glosses := []string{}
	next := 0
	for _, a := range line.Analysis {
		if a.FirstToken < next || a.LastToken < a.FirstToken || a.LastToken >= len(line.Tokens) {
			continue
		}
		if a.FirstToken > next {
			pieces = append(pieces, LinePiece{Ruby: rubyOf(line.Tokens[next:a.FirstToken]), Word: -1})
		}
		w := explainWord(dict, entries, line.Tokens, a)
		w.Short = wordGloss(w)
		pieces = append(pieces, LinePiece{Ruby: rubyOf(line.Tokens[a.FirstToken : a.LastToken+1]), Word: len(words)})
		words = append(words, w)
		if w.Short != "" {
			glosses = append(glosses, w.Short)
		}
		next = a.LastToken + 1
	}
	if next < len(line.Tokens) {
		pieces = append(pieces, LinePiece{Ruby: rubyOf(line.Tokens[next:]), Word: -1})
	}
	return words, pieces, strings.Join(glosses, " · ")
}

func explainWord(dict Dictionary, entries map[int64]Entry, tokens []Token, a WordAnalysis) Word {
	var text strings.Builder
	for _, t := range tokens[a.FirstToken : a.LastToken+1] {
		text.WriteString(t.Surface)
	}
	w := Word{
		FirstToken:   a.FirstToken,
		LastToken:    a.LastToken,
		Text:         text.String(),
		Lemma:        a.Lemma,
		LemmaReading: a.LemmaReading,
		Kind:         a.Kind,
		ConjClass:    a.ConjClass,
		Steps:        []GrammarPoint{},
	}
	if a.LemmaReading != "" {
		w.LemmaRomaji = jtext.Romaji(a.LemmaReading)
	}
	if a.EntryID != 0 {
		w.EntryID = a.EntryID
		sense := a.SenseIndex
		w.SenseIndex = &sense
		if e, ok := entries[a.EntryID]; ok {
			for _, s := range e.Senses {
				if s.Index == sense && len(s.Glosses) > 0 {
					w.Gloss = shortGloss(s.Glosses[0])
				}
			}
		}
	}
	if a.GrammarPointID != "" {
		g := grammarPoint(dict, a.GrammarPointID)
		w.Grammar = &g
	}
	for _, id := range a.Steps {
		w.Steps = append(w.Steps, grammarPoint(dict, id))
	}
	w.Form = formSentence(a.Lemma, a.Steps)
	return w
}

// grammarPoint returns the grammar point with the given ID. A point missing from the list, such as one a newer
// version of the app saved, shows its ID as its title.
func grammarPoint(dict Dictionary, id string) GrammarPoint {
	if g, ok := dict.GrammarPoint(id); ok {
		return g
	}
	return GrammarPoint{ID: id, Title: id}
}

// shortGloss trims a dictionary gloss for a line: notes in parentheses go, so "star (usu. excluding the Sun)" is
// "star", and it is cut to a length that fits under a word.
func shortGloss(gloss string) string {
	var b strings.Builder
	depth := 0
	for _, r := range gloss {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	short := strings.Join(strings.Fields(b.String()), " ")
	if short == "" {
		short = strings.TrimSpace(gloss)
	}
	if utf8.RuneCountInString(short) > maxGlossLen {
		short = string([]rune(short)[:maxGlossLen-1]) + "…"
	}
	return short
}

// wordGloss is a word's part of the line's gloss: the meaning of a content word without "to" ("shine"), a short
// English word for a particle where one fits ("'s", "in"), and otherwise the grammar title in parentheses. A word
// the dictionary doesn't explain shows its dictionary form, or nothing when that is the word itself.
func wordGloss(w Word) string {
	// Without a meaning, a word can only show itself again, which adds nothing under it.
	if g := glossOf(w); g != w.Text {
		return g
	}
	return ""
}

func glossOf(w Word) string {
	if w.Grammar != nil {
		if g, ok := grammarGlosses[w.Grammar.ID]; ok {
			return g
		}
		return "(" + strings.ToLower(w.Grammar.Title) + ")"
	}
	if w.Kind == WordAuxiliary {
		if g, ok := auxiliaryGlosses[w.Lemma]; ok {
			return g
		}
		return w.Lemma
	}
	if w.Gloss != "" {
		return strings.TrimPrefix(w.Gloss, "to ")
	}
	return w.Lemma
}

// grammarGlosses are the short English words particles and set phrases take in a line's gloss. A particle with no
// English word, such as が, shows its grammar title instead.
var grammarGlosses = map[string]string{
	"ga-subject": "(subject)", "ga-but": "but", "wa-topic": "(topic)", "wa-contrast": "(as for)",
	"wo-object": "(object)", "wo-through": "through", "ni-place": "in", "ni-time": "at", "ni-target": "to",
	"ni-destination": "to", "ni-purpose": "to", "ni-change": "into", "ni-agent": "by", "de-place": "at",
	"de-means": "with", "he-direction": "toward", "to-and": "and", "to-with": "with", "to-quote": "(quote)",
	"to-when": "when", "tte-quote": "(quote)", "no-possession": "'s", "no-nominalizer": "(the one that)",
	"no-question": "(?)", "mo-also": "also", "demo-even": "even", "sae-even": "even", "koso-emphasis": "(indeed)",
	"kara-from": "from", "kara-because": "because", "node-because": "so", "noni-although": "even though",
	"shi-and": "and", "made-until": "until", "yori-than": "than", "ya-and-so-on": "and", "toka-like": "or such",
	"nado-and-so-on": "and so on", "nanka-casual": "(like)", "tari-and": "(and so on)", "ka-question": "(?)",
	"ka-some": "some-", "ne-agreement": "(right?)", "yo-emphasis": "(!)", "zo-emphasis": "(!)",
	"sa-emphasis": "(you know)", "wa-feeling": "(!)", "na-feeling": "(how…)", "na-prohibition": "don't",
	"kana-wonder": "I wonder", "kedo-but": "but", "dake-only": "only", "shika-only": "only",
	"bakari-only": "nothing but", "hodo-extent": "so much", "kurai-about": "about", "te-mo": "even if",
	"ja-nai": "is not", "n-da": "(it's that)", "darou": "probably", "te-form": "and", "conditional-ba": "if",
	"nagara": "while",
}

var auxiliaryGlosses = map[string]string{"だ": "is", "です": "is"}

// stepLabels name conjugation steps for the sentence under a word.
var stepLabels = map[string]string{ //nolint:gosec // G101: the key "passive" is a grammar term, not a password.
	"te-form": "〜て", "teiru": "〜ている", "teru": "〜てる", "past": "〜た", "negative": "〜ない", "nu": "〜ぬ",
	"zu": "〜ず", "polite-masu": "〜ます", "continuative": "stem", "potential": "potential",
	"ra-nuki": "potential (without ら)", "passive": "passive", "causative": "causative",
	"causative-passive": "causative passive", "volitional": "volitional", "conditional-ba": "〜ば",
	"conditional-tara": "〜たら", "imperative": "command", "tai-want": "〜たい", "te-shimau": "〜てしまう",
	"chau": "〜ちゃう", "adverbial": "adverb", "na-attributive": "〜な", "te-iku": "〜ていく", "te-kuru": "〜てくる",
	"te-kureru": "〜てくれる", "sou-looks": "〜そう",
}

// formSentence says in plain words how a word is conjugated: "探す, in the 〜ている form", or "食べる, in the
// causative + passive + 〜ない + 〜た form". The te form isn't named when the next step already holds its て.
func formSentence(lemma string, steps []string) string {
	var labels []string
	for i, id := range steps {
		if id == "te-form" && i+1 < len(steps) {
			switch steps[i+1] {
			case "teiru", "teru", "te-shimau", "te-iku", "te-kuru", "te-kureru":
				continue
			}
		}
		label, ok := stepLabels[id]
		if !ok {
			label = id
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return ""
	}
	return lemma + ", in the " + strings.Join(labels, " + ") + " form"
}

// Meanings is everything the dictionary says about one word, for the word sheet. Installed is false when the
// dictionary of meanings isn't installed, and then nothing else is set. EntryID is 0 when the dictionary doesn't
// have the word.
type Meanings struct {
	Installed   bool      `json:"installed"`
	EntryID     int64     `json:"entryId"`
	Senses      []Meaning `json:"senses"`
	Attribution string    `json:"attribution"`
}

// Meaning is one sense of a word. PartsOfSpeech and Notes are the dictionary's codes put in words, such as "Godan
// verb with 'su' ending" and "usually written using kana alone".
type Meaning struct {
	Index         int      `json:"index"`
	Glosses       []string `json:"glosses"`
	PartsOfSpeech []string `json:"partsOfSpeech"`
	Notes         []string `json:"notes"`
}

// LookupWord returns every meaning the dictionary has for a word: the entry with the given ID when the lemma finds
// it, such as the entry a word in a song points at, and otherwise the most common entry for the lemma.
func LookupWord(ctx context.Context, dict Dictionary, lemma, reading string, entryID int64) (Meanings, error) {
	lemma = strings.TrimSpace(jtext.Normalize(lemma))
	reading = strings.TrimSpace(jtext.Normalize(reading))
	switch {
	case lemma == "":
		return Meanings{}, fault.Invalid("lemma", "is required")
	case utf8.RuneCountInString(lemma) > maxLookupLen:
		return Meanings{}, tooLong("lemma", maxLookupLen)
	case utf8.RuneCountInString(reading) > maxLookupLen:
		return Meanings{}, tooLong("reading", maxLookupLen)
	case reading != "" && !jtext.IsKana(reading):
		return Meanings{}, fault.Invalid("reading", "must be kana")
	case entryID < 0:
		return Meanings{}, fault.Invalid("entryId", "must not be negative")
	}

	entries, err := dict.Lookup(ctx, lemma, reading)
	if err == nil && len(entries) == 0 && reading != "" {
		entries, err = dict.Lookup(ctx, lemma, "")
	}
	if errors.Is(err, ErrNoMeanings) {
		return Meanings{Senses: []Meaning{}}, nil
	}
	if err != nil {
		return Meanings{}, err
	}
	found := Meanings{Installed: true, Senses: []Meaning{}, Attribution: dict.Attribution()}
	if len(entries) == 0 {
		return found, nil
	}
	entry := entries[0]
	for _, e := range entries {
		if e.ID == entryID {
			entry = e
		}
	}

	var codes []string
	for _, s := range entry.Senses {
		for _, c := range slices.Concat(s.PartsOfSpeech, s.Misc) {
			if !slices.Contains(codes, c) {
				codes = append(codes, c)
			}
		}
	}
	tags, err := dict.Tags(ctx, codes)
	if err != nil {
		return Meanings{}, err
	}
	describe := func(list []string) []string {
		out := make([]string, 0, len(list))
		for _, c := range list {
			if d, ok := tags[c]; ok {
				out = append(out, d)
			} else {
				out = append(out, c)
			}
		}
		return out
	}

	found.EntryID = entry.ID
	for _, s := range entry.Senses {
		found.Senses = append(found.Senses, Meaning{
			Index:         s.Index,
			Glosses:       append([]string{}, s.Glosses...),
			PartsOfSpeech: describe(s.PartsOfSpeech),
			Notes:         describe(s.Misc),
		})
	}
	return found, nil
}
