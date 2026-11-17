// Package morph splits Japanese text into tokens with their readings and parts of speech, using the kagome
// morphological analyzer and the IPA dictionary.
package morph

import (
	"fmt"
	"sync"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

// POS is a coarse part of speech. The codes are stable: they are stored in line_tokens.pos, whose CHECK lists them.
type POS string

const (
	Noun         POS = "noun"
	Pronoun      POS = "pronoun"
	Verb         POS = "verb"
	Adjective    POS = "adjective"
	Adverb       POS = "adverb"
	Particle     POS = "particle"
	Auxiliary    POS = "auxiliary"
	Prefix       POS = "prefix"
	Suffix       POS = "suffix"
	Conjunction  POS = "conjunction"
	Interjection POS = "interjection"
	Symbol       POS = "symbol"
	Other        POS = "other"
)

// AllPOS returns every part of speech code.
func AllPOS() []POS {
	return []POS{Noun, Pronoun, Verb, Adjective, Adverb, Particle, Auxiliary, Prefix, Suffix, Conjunction,
		Interjection, Symbol, Other}
}

// Token is one piece of analysed text. Surface is exactly as written, so a text's surfaces join back into it.
// Reading is hiragana, empty for punctuation, Latin text, numbers, and words the dictionary has no reading for.
// The other fields are the IPA dictionary's own, empty where it has none: POSDetail is the first sub-category
// (格助詞), POSDetail2 the second (引用, for a と that quotes), BaseForm the dictionary form (探す), ConjType and ConjForm how the word conjugates and the form used here
// (五段・サ行, 連用形), and Pronunciation how it is said, in katakana (お母さん is オカーサン).
type Token struct {
	Surface       string
	Reading       string
	POS           POS
	POSDetail     string
	POSDetail2    string
	BaseForm      string
	ConjType      string
	ConjForm      string
	Pronunciation string
}

// Attribution is the credit the IPA dictionary's licence asks for: its source, and the no-warranty notice that must
// ship with every copy.
func Attribution() string {
	return "Word boundaries and readings come from the IPA dictionary (mecab-ipadic 2.7.0-20070801), copyright the " +
		"Nara Institute of Science and Technology (NAIST), with many entries from ICOT Free Software, through " +
		"kagome and kagome-dict by ikawaha (MIT licence). NO WARRANTY: the dictionary is provided as is, without " +
		"any warranty whatsoever, and NAIST, ICOT, and everyone involved in making it disclaim all warranties and " +
		"are not liable for any damages arising from its use."
}

// Analyzer splits text into tokens. The dictionary takes about 145 MB of memory, so it is loaded on the first call,
// never at startup, and kept for the life of the Analyzer. It is safe for concurrent use.
type Analyzer struct {
	load func() (*tokenizer.Tokenizer, error)
}

// New returns an Analyzer. It doesn't load the dictionary yet.
func New() *Analyzer {
	return &Analyzer{load: sync.OnceValues(func() (*tokenizer.Tokenizer, error) {
		t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
		if err != nil {
			return nil, fmt.Errorf("morph: load IPA dictionary failed (%v)", err)
		}
		return t, nil
	})}
}

// Analyze splits text into tokens. The first call loads the dictionary, which takes a moment.
func (a *Analyzer) Analyze(text string) ([]Token, error) {
	t, err := a.load()
	if err != nil {
		return nil, err
	}
	raw := t.Tokenize(text)
	tokens := make([]Token, 0, len(raw))
	for _, r := range raw {
		tokens = append(tokens, token(r))
	}
	return tokens, nil
}

// IPA features are POS, POS1, POS2, POS3, conjugation type, conjugation form, base form, reading, and pronunciation.
// Words the dictionary doesn't know have only the first seven.
func token(r tokenizer.Token) Token {
	f := r.Features()
	tok := Token{
		Surface:       r.Surface,
		POS:           coarsePOS(field(f, 0), field(f, 1)),
		POSDetail:     field(f, 1),
		POSDetail2:    field(f, 2),
		ConjType:      field(f, ipa.InflectionalType),
		ConjForm:      field(f, ipa.InflectionalForm),
		BaseForm:      field(f, ipa.BaseForm),
		Pronunciation: field(f, ipa.Pronunciation),
	}
	reading := jtext.ToHiragana(field(f, ipa.Reading))
	if reading == "" && jtext.IsKana(r.Surface) {
		// A kana word the dictionary doesn't know, such as ケータイ, still reads as written.
		reading = jtext.ToHiragana(r.Surface)
	}
	if jtext.IsHiragana(reading) {
		tok.Reading = reading
	}
	return tok
}

// field returns feature i, or "" when the token has no such feature or the dictionary leaves it blank (*).
func field(f []string, i int) string {
	if i >= len(f) || f[i] == "*" {
		return ""
	}
	return f[i]
}

func coarsePOS(pos, detail string) POS {
	switch pos {
	case "名詞":
		switch detail {
		case "代名詞":
			return Pronoun
		case "接尾":
			return Suffix
		}
		return Noun
	case "動詞":
		if detail == "接尾" {
			return Suffix
		}
		return Verb
	case "形容詞":
		if detail == "接尾" {
			return Suffix
		}
		return Adjective
	case "副詞":
		return Adverb
	case "助詞":
		return Particle
	case "助動詞":
		return Auxiliary
	case "接頭詞":
		return Prefix
	case "接続詞":
		return Conjunction
	case "感動詞", "フィラー":
		return Interjection
	case "記号":
		return Symbol
	default:
		// 連体詞 (この, 大きな), その他, and anything new.
		return Other
	}
}
