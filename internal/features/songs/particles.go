package songs

import (
	"slices"
	"strings"
	"unicode"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// particleUse guesses what particle word k does in the line, as a grammar point ID, from the tokens around it. Each
// particle falls back to its most common use. It gives "" for a particle with no grammar point.
func particleUse(tokens []Token, words []WordAnalysis, k int) string {
	w := words[k]
	t := tokens[w.FirstToken]
	var prev, next Token
	if w.FirstToken > 0 {
		prev = tokens[w.FirstToken-1]
	}
	if w.LastToken+1 < len(tokens) {
		next = tokens[w.LastToken+1]
	}

	switch t.base() {
	case "が":
		if t.Detail == "接続助詞" {
			return "ga-but"
		}
		return "ga-subject"
	case "は":
		return "wa-topic"
	case "を":
		return "wo-object"
	case "に":
		return niUse(tokens, w, prev, next)
	case "で":
		if t.Detail == "接続助詞" {
			return "te-form"
		}
		return "de-place"
	case "へ":
		return "he-direction"
	case "と":
		return toUse(t, prev, next)
	case "って":
		return "tte-quote"
	case "の", "ん":
		switch {
		case t.POS == morph.Noun:
			return "no-nominalizer"
		case t.base() == "ん":
			return ""
		case t.Detail == "終助詞":
			return "no-question"
		case next.POS == morph.Verb || isNai(next) || (next.POS == morph.Adjective && next.base() == "なき"):
			// A verb or ない after の means の stands for が in a describing phrase: 変わることのない愛.
			return "no-subject"
		}
		return "no-possession"
	case "も":
		if k > 0 && endsInTe(tokens, words[k-1]) && words[k-1].LastToken+1 == w.FirstToken {
			return "te-mo"
		}
		return "mo-also"
	case "から":
		if t.Detail == "接続助詞" || prev.POS == morph.Verb || prev.POS == morph.Adjective ||
			prev.POS == morph.Auxiliary {
			return "kara-because"
		}
		return "kara-from"
	case "か":
		if prev.POS == morph.Pronoun && slices.Contains(questionWords, prev.base()) {
			return "ka-some"
		}
		return "ka-question"
	case "な", "なあ", "なぁ":
		if t.Detail == "終助詞" && prev.POS == morph.Verb && prev.ConjForm == "基本形" {
			return "na-prohibition"
		}
		return "na-feeling"
	case "よ":
		switch {
		case w.LastToken > w.FirstToken:
			// よか is spoken より: 死ぬよか, rather than dying.
			return "yori-than"
		case (prev.POS == morph.Noun || prev.POS == morph.Pronoun) && callsOut(tokens, w.LastToken+1):
			return "yo-calling"
		}
		return "yo-emphasis"
	case "でも":
		if t.Detail == "接続助詞" {
			return "te-mo"
		}
		return "demo-even"
	case "たって", "だって":
		if t.Detail == "接続助詞" {
			return "te-mo"
		}
		return "demo-even"
	}
	return particleUses[t.base()]
}

// callsOut reports whether よ after a noun calls out to it (鏡よ鏡よ, 星よ) rather than stressing the sentence: the
// line goes on after it, past any spaces or commas, with a word that isn't another particle.
func callsOut(tokens []Token, from int) bool {
	for _, t := range tokens[from:] {
		if t.POS == morph.Symbol || strings.TrimFunc(t.Surface, unicode.IsSpace) == "" {
			continue
		}
		return t.POS != morph.Particle && t.POS != morph.Auxiliary
	}
	return false
}

// particleUses are the particles with one use the app knows.
var particleUses = map[string]string{
	"まで": "made-until", "より": "yori-than", "や": "ya-and-so-on", "ね": "ne-agreement", "ねえ": "ne-agreement",
	"ねぇ": "ne-agreement", "かな": "kana-wonder", "かしら": "kana-wonder", "けど": "kedo-but",
	"けれど": "kedo-but", "けれども": "kedo-but", "だけ": "dake-only", "しか": "shika-only", "のに": "noni-although",
	"ので": "node-because", "し": "shi-and", "ても": "te-mo", "さえ": "sae-even", "こそ": "koso-emphasis",
	"ばかり": "bakari-only", "ばっかり": "bakari-only", "ばっか": "bakari-only", "など": "nado-and-so-on",
	"なんか": "nanka-casual", "なんて": "nanka-casual", "とか": "toka-like", "ほど": "hodo-extent",
	"くらい": "kurai-about", "ぐらい": "kurai-about", "たり": "tari-and", "だり": "tari-and", "ぞ": "zo-emphasis",
	"ぜ": "zo-emphasis", "さ": "sa-emphasis", "わ": "wa-feeling", "ながら": "nagara", "て": "te-form",
	"ば": "conditional-ba",
}

var questionWords = []string{"何", "なに", "なん", "誰", "だれ", "どこ", "いつ", "どれ", "どちら", "どう"}

// niUse picks the use of に. A verb stem before it and 行く or 来る after it is a purpose (会いに行く). A time word
// before it is a point in time (夜に): kagome marks those as nouns that can be adverbs, so a few place words with that
// mark are left out. なる after it is a change (大人になる), a person before it is the target (君に会う), and a verb of
// moving after it is a destination (東京に行く). Anything else is where something is or happens.
func niUse(tokens []Token, w WordAnalysis, prev, next Token) string {
	if next.POS == morph.Verb && slices.Contains(comingAndGoing, next.base()) &&
		((prev.POS == morph.Verb && prev.ConjForm == "連用形") || prev.Detail == "サ変接続") {
		return "ni-purpose"
	}
	if next.POS == morph.Verb && next.base() == "なる" {
		return "ni-change"
	}
	if prev.POS == morph.Noun && (prev.Detail == "副詞可能" || prev.Detail == "数" || prev.Detail2 == "助数詞" ||
		prev.Detail2 == "副詞可能") && !slices.Contains(placeWords, prev.base()) {
		return "ni-time"
	}
	if (prev.POS == morph.Pronoun && !slices.Contains(placeWords, prev.base())) || prev.Detail2 == "人名" {
		return "ni-target"
	}
	for _, t := range tokens[w.LastToken+1:] {
		if t.POS == morph.Verb && t.Detail == "自立" {
			if slices.Contains(movingVerbs, t.base()) {
				return "ni-destination"
			}
			break
		}
	}
	return "ni-place"
}

var comingAndGoing = []string{"行く", "いく", "来る", "くる", "帰る", "戻る", "出かける"}

var movingVerbs = []string{
	"行く", "いく", "来る", "くる", "帰る", "戻る", "向かう", "着く", "入る", "届く", "届ける", "落ちる", "乗る",
}

// placeWords are nouns and pronouns of place, which kagome marks like time words or people.
var placeWords = []string{
	"上", "下", "中", "前", "後ろ", "横", "隣", "側", "外", "内", "奥", "向こう", "周り", "そば", "傍", "間",
	"ここ", "そこ", "あそこ", "どこ", "こちら", "そちら", "あちら", "どちら", "こっち", "そっち", "あっち", "どっち",
}

// toUse picks the use of と: a quote when kagome marks it so or a closing quote mark comes before it, "when" after a
// verb that ends a clause, "and" between two nouns, and "with" otherwise.
func toUse(t, prev, next Token) string {
	switch {
	case t.Detail == "接続助詞":
		return "to-when"
	case t.Detail2 == "引用" || prev.Detail == "括弧閉":
		return "to-quote"
	case t.Detail == "並立助詞" || next.POS == morph.Noun || next.POS == morph.Pronoun:
		return "to-and"
	}
	return "to-with"
}

func endsInTe(tokens []Token, w WordAnalysis) bool {
	last := tokens[w.LastToken]
	return w.Kind == WordContent && last.POS == morph.Particle && (last.base() == "て" || last.base() == "で")
}
