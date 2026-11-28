package songs

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// WordKind sorts the words of a line. The codes are stored in line_words.kind.
type WordKind string

const (
	// WordContent is a noun, verb, adjective, adverb, or other word with a meaning of its own.
	WordContent WordKind = "content"
	// WordParticle is a particle, with the use it has in the line.
	WordParticle WordKind = "particle"
	// WordExpression is a set phrase, such as じゃない or んだ.
	WordExpression WordKind = "expression"
	// WordAuxiliary is a grammar word that stands alone, such as だ or です after a noun.
	WordAuxiliary WordKind = "auxiliary"
)

// Who made an analysis. The analyzer's guesses are auto; a correction you make is user, and is never overwritten.
const (
	AnalysisAuto = "auto"
	AnalysisUser = "user"
)

const (
	maxLemmaLen        = 100
	maxLemmaReadingLen = 200
)

// WordAnalysis is one stored word of a line: the tokens it covers, from FirstToken to LastToken, and what the analysis
// says about it. Lemma is the dictionary form (探す for 探している), ConjClass how it conjugates as a JMdict code, and
// Steps the conjugations, as grammar point IDs in the order they apply. EntryID is 0 when the word has no dictionary
// entry, and SenseIndex is the meaning used in the line, set only with an entry.
type WordAnalysis struct {
	ID             int64
	FirstToken     int
	LastToken      int
	Kind           WordKind
	Lemma          string
	LemmaReading   string
	ConjClass      string
	EntryID        int64
	SenseIndex     int
	GrammarPointID string
	Steps          []string
	Source         string

	// altLemma and altReading are a shorter form to look up when the dictionary doesn't have the whole lemma: the
	// noun of a noun and する (勉強 for 勉強する), or the word without its prefix or suffix (君 for 君たち).
	altLemma   string
	altReading string
	// head is the position of the token that carries the word's meaning, -1 when it isn't known.
	head int
}

// findWords groups a line's tokens into words. A content word takes the suffixes, auxiliaries, and conjugation pieces
// that attach to it, so 探し, て, いる is one word, 探す in the 〜ている form. Particles are words of their own.
// Punctuation, symbols, spaces, and text with no Japanese in it (English, numbers) belong to no word. It needs the analyzer's IPA fields; tokens read back from the
// database don't have them.
func findWords(tokens []Token) []WordAnalysis {
	var words []WordAnalysis
	for i := 0; i < len(tokens); {
		if numberBeforeCounter(tokens, i) {
			w := numberWord(tokens, i)
			words = append(words, w)
			i = w.LastToken + 1
			continue
		}
		if outsideWords(tokens[i]) {
			i++
			continue
		}
		w := takeWord(tokens, i)
		words = append(words, w)
		i = w.LastToken + 1
	}
	for k := range words {
		if words[k].Kind != WordParticle {
			continue
		}
		words[k].GrammarPointID = particleUse(tokens, words, k)
		if words[k].GrammarPointID == "" {
			// No grammar point fits, and the database requires one for a particle, so it stands as a grammar word.
			words[k].Kind = WordAuxiliary
		}
	}
	return words
}

func outsideWords(t Token) bool {
	return t.POS == morph.Symbol || !hasJapanese(t.Surface)
}

// hasJapanese reports whether s has kana or kanji in it. An English phrase in a Japanese song stays as it's sung,
// not cut into words that only repeat themselves.
func hasJapanese(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
	})
}

// numberBeforeCounter reports whether token i is a number written in digits with a counter after it, as in 1番 or 2人.
// The two are one word, though digits alone belong to no word.
func numberBeforeCounter(tokens []Token, i int) bool {
	if i+1 >= len(tokens) || tokens[i+1].POS != morph.Suffix {
		return false
	}
	return tokens[i].Surface != "" && strings.IndexFunc(tokens[i].Surface, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

// numberWord makes a number and its counter one word. JMdict spells numbers in kanji (一番, 二人), so the lemma does
// too when the number is small enough to have one kanji; otherwise the counter alone is looked up.
func numberWord(tokens []Token, i int) WordAnalysis {
	counter := tokens[i+1]
	w := WordAnalysis{FirstToken: i, LastToken: i + 1, Kind: WordContent, Source: AnalysisAuto, head: i + 1}
	digits := tokens[i].Surface
	if k, ok := kanjiDigits[strings.TrimLeft(norm.NFKC.String(digits), "0")]; ok {
		w.Lemma, _ = cleanLemma(k+counter.base(), "")
	} else {
		w.Lemma, _ = cleanLemma(digits+counter.base(), "")
	}
	w.altLemma, w.altReading = cleanLemma(counter.base(), counter.Reading)
	return w
}

var kanjiDigits = map[string]string{
	"1": "一", "2": "二", "3": "三", "4": "四", "5": "五", "6": "六", "7": "七", "8": "八", "9": "九", "10": "十",
}

// wordDraft is a word while its tokens are being gathered.
type wordDraft struct {
	tokens []Token
	w      WordAnalysis
	// stem is the token whose conjugation form the pieces after it build on: the head, or する after a noun.
	stem int
	// conjugates is true once the word can take conjugation pieces: a verb, an adjective, a な-adjective with its
	// copula, or a grammar word.
	conjugates bool
	// stepsFrom is the first token whose pieces count as conjugation steps.
	stepsFrom int
}

func takeWord(tokens []Token, i int) WordAnalysis {
	d := wordDraft{tokens: tokens, stem: i, stepsFrom: i + 1}
	d.w = WordAnalysis{FirstToken: i, LastToken: i, Kind: WordContent, Source: AnalysisAuto, head: i}
	t := tokens[i]

	switch {
	case isNominalizer(t) && i+1 < len(tokens) && isCopula(tokens[i+1]):
		// 行くんだ: ん and だ together explain or insist.
		d.expression(i+1, t.Surface+"だ", t.Reading+"だ", "n-da")
	case isNominalizer(t):
		d.w.Kind = WordParticle
	case (t.base() == "じゃ" || t.base() == "では") && t.POS == morph.Particle && i+1 < len(tokens) &&
		isNai(tokens[i+1]):
		d.expression(i+1, t.Surface+"ない", t.Reading+"ない", "ja-nai")
	case t.POS == morph.Pronoun && slices.Contains(questionWords, t.base()) && demoAfter(tokens, i+1) > 0:
		// A question word and でも are one word, any-: なんでも, anything; だれでも, anyone.
		d.w.LastToken = i + demoAfter(tokens, i+1)
		d.stepsFrom = d.w.LastToken + 1
		d.w.Lemma, d.w.LemmaReading = t.Surface+"でも", t.Reading+"でも"
		d.w.altLemma, d.w.altReading = t.base(), lemmaReading(t)
	case t.POS == morph.Other && i+1 < len(tokens) && (tokens[i+1].base() == "まま" || tokens[i+1].base() == "儘"):
		// このまま, そのまま: a word of its own in the dictionary, "as it is".
		d.w.LastToken = i + 1
		d.stepsFrom = i + 2
		d.w.Lemma, d.w.LemmaReading = t.Surface+tokens[i+1].Surface, t.Reading+tokens[i+1].Reading
	case t.POS == morph.Particle && t.base() == "で" && i+1 < len(tokens) && tokens[i+1].POS == morph.Particle &&
		tokens[i+1].base() == "も" && i+3 < len(tokens) && (tokens[i+2].POS == morph.Noun ||
		tokens[i+2].POS == morph.Pronoun) && demoAfter(tokens, i+3) > 0:
		// 針でもなんでも, needles or anything: with a second でも straight after, で and も are でも, not "at" and
		// "also" as in 学校でも.
		d.w.Kind = WordParticle
		d.w.LastToken = i + 1
		d.w.Lemma, d.w.LemmaReading = "でも", "でも"
	case t.POS == morph.Particle && t.base() == "よ" && i+1 < len(tokens) && tokens[i+1].POS == morph.Particle &&
		tokens[i+1].base() == "か":
		d.w.Kind = WordParticle
		d.w.LastToken = i + 1
		d.w.Lemma, d.w.LemmaReading = "よか", "よか"
	case t.POS == morph.Particle:
		d.w.Kind = WordParticle
	case t.POS == morph.Auxiliary:
		d.w.Kind = WordAuxiliary
		d.conjugates = true
		if isCopula(t) && t.ConjForm == "未然形" && i+1 < len(tokens) && tokens[i+1].base() == "う" {
			// だろう and でしょう make a guess; they are one grammar word, not a copula made volitional.
			d.w.GrammarPointID = "darou"
			d.w.LastToken = i + 1
			d.stepsFrom = i + 2
		}
	case t.POS == morph.Prefix && i+1 < len(tokens) && takesPrefix(tokens[i+1]):
		d.w.head, d.stem, d.stepsFrom = i+1, i+1, i+2
		d.w.LastToken = i + 1
		d.conjugates = conjugatingHead(tokens[i+1])
	default:
		d.conjugates = conjugatingHead(t)
	}
	d.gather()
	d.name()
	d.w.Steps = d.steps()
	return d.w
}

// demoAfter counts the tokens of でも at position i: 1 for でも, 2 when kagome reads it as で and も, 0 for none.
func demoAfter(tokens []Token, i int) int {
	particle := func(j int, base string) bool {
		return j < len(tokens) && tokens[j].POS == morph.Particle && tokens[j].base() == base
	}
	switch {
	case particle(i, "でも"):
		return 1
	case particle(i, "で") && particle(i+1, "も"):
		return 2
	}
	return 0
}

func (d *wordDraft) expression(last int, lemma, reading, grammar string) {
	d.w.Kind = WordExpression
	d.w.LastToken = last
	d.w.Lemma, d.w.LemmaReading = lemma, reading
	d.w.GrammarPointID = grammar
	d.stem, d.stepsFrom = last, last+1
	d.conjugates = true
}

// gather adds the tokens that attach to the word, one by one, until one doesn't.
func (d *wordDraft) gather() {
	if d.w.Kind == WordParticle {
		return
	}
	for j := d.w.LastToken + 1; j < len(d.tokens); j++ {
		if !d.attaches(j) {
			return
		}
		d.w.LastToken = j
	}
}

func (d *wordDraft) attaches(j int) bool {
	t, prev := d.tokens[j], d.tokens[j-1]
	head := d.tokens[d.w.head]
	if outsideWords(t) {
		return false
	}
	// A noun's suffix (君たち, 二人) and a verb's (食べさせる, 降りそう) both belong to the word.
	if t.POS == morph.Suffix && d.w.Kind == WordContent {
		return true
	}
	if j == d.w.head+1 && d.w.Kind == WordContent {
		switch {
		case head.Detail == "形容動詞語幹" && (isCopula(t) || (t.POS == morph.Particle && t.Detail == "副詞化")):
			// 静かな, 静かに, 静かだった: a な-adjective takes its copula.
			d.conjugates = true
			return true
		case (head.Detail == "サ変接続" || head.Detail == "一般" || head.Detail == "固有名詞") && t.POS == morph.Verb &&
			t.base() == "する":
			// 勉強する and おサラバする are one verb: a noun with する straight after it is a する verb.
			d.stem = j
			d.stepsFrom = j + 1
			d.conjugates = true
			return true
		}
	}
	if !d.conjugates {
		return false
	}
	if prev.POS == morph.Particle && prev.base() == "ば" {
		return false
	}
	switch t.POS {
	case morph.Auxiliary:
		return conjugationAuxiliary(t)
	case morph.Particle:
		return t.Detail == "接続助詞" && (t.base() == "て" || t.base() == "で" || t.base() == "ば")
	case morph.Verb, morph.Adjective:
		return t.Detail == "非自立" || isNai(t)
	}
	return false
}

// conjugationAuxiliary reports whether an auxiliary conjugates the word before it (ない, た, ます, たい, う), rather than
// standing apart as a grammar word of its own (だ, です, らしい).
func conjugationAuxiliary(t Token) bool {
	switch t.ConjType {
	case "特殊・タ", "特殊・ナイ", "特殊・ヌ", "特殊・マス", "特殊・タイ":
		return true
	case "不変化型":
		switch t.base() {
		case "う", "よう", "ん", "まい":
			return true
		}
	}
	return false
}

func isCopula(t Token) bool {
	return t.POS == morph.Auxiliary && (t.ConjType == "特殊・ダ" || t.ConjType == "特殊・デス")
}

func isNai(t Token) bool {
	return t.base() == "ない" && (t.POS == morph.Auxiliary || t.POS == morph.Adjective)
}

// isNominalizer reports whether t is the の (or its short form ん) that turns a phrase into a noun.
func isNominalizer(t Token) bool {
	return t.POS == morph.Noun && t.Detail == "非自立" && (t.base() == "の" || t.base() == "ん")
}

func takesPrefix(t Token) bool {
	switch t.POS {
	case morph.Noun, morph.Verb, morph.Adjective, morph.Pronoun:
		return true
	}
	return false
}

func conjugatingHead(t Token) bool {
	return t.POS == morph.Verb || t.POS == morph.Adjective
}

// name sets the word's lemma, its reading, and how it conjugates.
func (d *wordDraft) name() {
	if d.w.Lemma != "" {
		d.w.Lemma, d.w.LemmaReading = cleanLemma(d.w.Lemma, d.w.LemmaReading)
		return
	}
	head := d.tokens[d.w.head]
	lemma := spelling{text: head.base(), reading: lemmaReading(head)}
	var prefix spelling
	if d.w.head > d.w.FirstToken {
		p := d.tokens[d.w.FirstToken]
		prefix = spelling{text: p.Surface, reading: p.Reading}
	}

	var alt spelling
	switch {
	case d.stem != d.w.head:
		// A noun and する: 勉強する conjugates as a する verb. A prefix stays with the noun: おサラバ.
		alt = prefix.join(lemma)
		lemma = alt.join(spelling{text: "する", reading: "する"})
		d.w.ConjClass = "vs"
	case head.POS == morph.Noun || head.POS == morph.Pronoun:
		var suffix spelling
		for j := d.w.head + 1; j <= d.w.LastToken; j++ {
			if t := d.tokens[j]; t.POS == morph.Suffix {
				suffix = suffix.join(spelling{text: t.base(), reading: t.Reading})
			}
		}
		if prefix.text != "" || suffix.text != "" {
			alt = lemma
			lemma = prefix.join(lemma).join(suffix)
		}
		if head.Detail == "形容動詞語幹" {
			d.w.ConjClass = "adj-na"
		}
	default:
		lemma = prefix.join(lemma)
		d.w.ConjClass = conjClass(head)
	}
	d.w.Lemma, d.w.LemmaReading = cleanLemma(lemma.text, lemma.reading)
	d.w.altLemma, d.w.altReading = cleanLemma(alt.text, alt.reading)
}

// spelling is a piece of a lemma with its reading. A piece with text but no reading makes the whole reading unknown.
type spelling struct {
	text    string
	reading string
}

func (s spelling) join(o spelling) spelling {
	switch {
	case s.text == "":
		return o
	case o.text == "":
		return s
	case s.reading == "" || o.reading == "":
		return spelling{text: s.text + o.text}
	}
	return spelling{text: s.text + o.text, reading: s.reading + o.reading}
}

// cleanLemma keeps the lemma within the database's limits, and drops a reading that isn't hiragana.
func cleanLemma(lemma, reading string) (string, string) {
	if utf8.RuneCountInString(lemma) > maxLemmaLen {
		lemma = string([]rune(lemma)[:maxLemmaLen])
	}
	if !jtext.IsHiragana(reading) || utf8.RuneCountInString(reading) > maxLemmaReadingLen {
		reading = ""
	}
	return lemma, reading
}

// lemmaReading works out the reading of a token's dictionary form from the reading of its surface: 探し read さがし
// with the dictionary form 探す is さがす. The kana the two forms don't share is swapped at the end of the reading. It
// gives "" when the two don't line up.
func lemmaReading(t Token) string {
	base := t.base()
	if base == t.Surface || t.Reading == "" {
		return t.Reading
	}
	if strings.HasPrefix(t.ConjType, "カ変") {
		// 来 is read こ, き, or く depending on the form, so the reading can't be swapped at the end.
		return "くる"
	}
	surface, lemma := []rune(t.Surface), []rune(base)
	same := 0
	for same < len(surface) && same < len(lemma) && surface[same] == lemma[same] {
		same++
	}
	surfaceTail := jtext.ToHiragana(string(surface[same:]))
	lemmaTail := jtext.ToHiragana(string(lemma[same:]))
	if !strings.HasSuffix(t.Reading, surfaceTail) || (lemmaTail != "" && !jtext.IsHiragana(lemmaTail)) {
		return ""
	}
	return strings.TrimSuffix(t.Reading, surfaceTail) + lemmaTail
}

// conjClass maps the IPA conjugation type of a verb or adjective to its JMdict code, or "" when there is none.
func conjClass(t Token) string {
	ct := t.ConjType
	switch {
	case ct == "":
		return ""
	case strings.HasPrefix(ct, "五段"):
		return godanClass(ct, t.base())
	case ct == "一段・クレル":
		return "v1-s"
	case strings.HasPrefix(ct, "一段"):
		return "v1"
	case strings.HasPrefix(ct, "カ変"):
		return "vk"
	case ct == "サ変・スル":
		return "vs-i"
	case strings.HasPrefix(ct, "サ変") && strings.HasSuffix(ct, "スル"):
		return "vs-s"
	case strings.HasPrefix(ct, "サ変") && strings.HasSuffix(ct, "ズル"):
		return "vz"
	case ct == "形容詞・イイ":
		return "adj-ix"
	case strings.HasPrefix(ct, "形容詞"):
		return "adj-i"
	}
	return ""
}

func godanClass(ct, base string) string {
	switch {
	case strings.HasPrefix(ct, "五段・カ行促音便"):
		return "v5k-s"
	case strings.HasPrefix(ct, "五段・カ行"):
		return "v5k"
	case strings.HasPrefix(ct, "五段・ガ行"):
		return "v5g"
	case strings.HasPrefix(ct, "五段・サ行"):
		return "v5s"
	case strings.HasPrefix(ct, "五段・タ行"):
		return "v5t"
	case strings.HasPrefix(ct, "五段・ナ行"):
		return "v5n"
	case strings.HasPrefix(ct, "五段・バ行"):
		return "v5b"
	case strings.HasPrefix(ct, "五段・マ行"):
		return "v5m"
	case ct == "五段・ラ行特殊":
		return "v5aru"
	case strings.HasPrefix(ct, "五段・ラ行"):
		if base == "ある" || base == "有る" || base == "在る" {
			return "v5r-i"
		}
		return "v5r"
	case ct == "五段・ワ行ウ音便":
		return "v5u-s"
	case strings.HasPrefix(ct, "五段・ワ行"):
		return "v5u"
	}
	return ""
}

// steps lists the word's conjugations, as grammar point IDs, from the pieces after its stem. Pieces it doesn't know
// add no step.
func (d *wordDraft) steps() []string {
	var steps []string
	add := func(ids ...string) { steps = append(steps, ids...) }
	stem := d.tokens[d.stem]
	for j := max(d.stepsFrom, d.stem+1); j <= d.w.LastToken; j++ {
		p, prev := d.tokens[j], d.tokens[j-1]
		switch p.POS {
		case morph.Suffix:
			if stem.POS == morph.Verb || stem.POS == morph.Adjective {
				add(suffixStep(p, prev)...)
			}
		case morph.Auxiliary:
			add(auxiliaryStep(p)...)
		case morph.Particle:
			switch {
			case p.base() == "て" || p.base() == "で":
				add("te-form")
			case p.base() == "ば":
				add("conditional-ba")
			case p.Detail == "副詞化":
				add("adverbial")
			}
		case morph.Verb:
			add(helperVerbStep(p, prev)...)
		case morph.Adjective:
			if isNai(p) {
				add("negative")
			}
		}
	}
	add(endingStep(d.tokens[d.w.LastToken], d.w.LastToken == d.stem)...)
	return steps
}

func suffixStep(p, prev Token) []string {
	switch p.base() {
	case "させる", "せる":
		return []string{"causative"}
	case "れる":
		// 見れる and 来れる: れる straight after an ichidan or kuru stem is the potential without ら.
		if prev.POS == morph.Verb && (strings.HasPrefix(prev.ConjType, "一段") || strings.HasPrefix(prev.ConjType, "カ変")) {
			return []string{"ra-nuki"}
		}
		return []string{"passive"}
	case "られる":
		// られる can be passive, potential, or respectful. Passive is the most common in lyrics.
		return []string{"passive"}
	case "そう":
		return []string{"sou-looks"}
	}
	return nil
}

func auxiliaryStep(p Token) []string {
	switch p.ConjType {
	case "特殊・タ":
		if p.ConjForm == "仮定形" {
			return []string{"conditional-tara"}
		}
		return []string{"past"}
	case "特殊・ナイ":
		return []string{"negative"}
	case "特殊・ヌ":
		if strings.HasPrefix(p.Surface, "ず") {
			return []string{"zu"}
		}
		return []string{"nu"}
	case "特殊・マス":
		return []string{"polite-masu"}
	case "特殊・タイ":
		return []string{"tai-want"}
	case "特殊・ダ":
		switch {
		case p.ConjForm == "体言接続":
			return []string{"na-attributive"}
		case p.Surface == "で":
			return []string{"te-form"}
		}
	case "不変化型":
		switch p.base() {
		case "う", "よう":
			return []string{"volitional"}
		case "ん":
			return []string{"negative"}
		}
	}
	return nil
}

// helperVerbStep names the helper verbs that follow the te form: いる, しまう, いく, くる, くれる, those in teHelpers,
// and the spoken short forms てる and ちゃう, which carry the て inside them.
func helperVerbStep(p, prev Token) []string {
	afterTe := prev.POS == morph.Particle && (prev.base() == "て" || prev.base() == "で")
	switch p.base() {
	case "いる":
		if afterTe {
			return []string{"teiru"}
		}
	case "てる", "でる":
		return []string{"te-form", "teru"}
	case "しまう":
		if afterTe {
			return []string{"te-shimau"}
		}
	case "ちゃう", "じゃう", "ちまう", "じまう":
		return []string{"chau"}
	case "いく", "ゆく":
		if afterTe {
			return []string{"te-iku"}
		}
	case "くる":
		if afterTe {
			return []string{"te-kuru"}
		}
	case "くれる":
		if afterTe {
			return []string{"te-kureru"}
		}
	default:
		if id, ok := teHelpers[p.base()]; ok && afterTe {
			return []string{id}
		}
	}
	return nil
}

// teHelpers are the other helper verbs after the te form, each with its own meaning: 治してみせる, I'll cure it, you'll
// see.
var teHelpers = map[string]string{
	"みせる": "te-miseru", "見せる": "te-miseru", "いただく": "te-itadaku", "頂く": "te-itadaku", "みる": "te-miru",
	"見る": "te-miru", "もらう": "te-morau", "貰う": "te-morau", "あげる": "te-ageru", "上げる": "te-ageru",
	"おく": "te-oku", "置く": "te-oku",
}

// endingStep names the form of the word's last token when nothing after it does: a command (走れ), an adjective
// used as an adverb (早く), or a verb stem standing alone (会いに, 光り).
func endingStep(last Token, alone bool) []string {
	switch {
	case (last.POS == morph.Verb || last.POS == morph.Adjective) && strings.HasPrefix(last.ConjForm, "命令"):
		return []string{"imperative"}
	case last.POS == morph.Adjective && last.ConjForm == "連用テ接続" && alone:
		return []string{"adverbial"}
	case last.POS == morph.Verb && last.ConjForm == "連用形" && alone:
		return []string{"continuative"}
	}
	return nil
}

// base is the token's dictionary form, or its surface when the analyzer doesn't know one.
func (t Token) base() string {
	if t.BaseForm != "" {
		return t.BaseForm
	}
	return t.Surface
}
