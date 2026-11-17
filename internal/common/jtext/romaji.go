package jtext

import (
	"strings"
	"unicode"
)

// Romaji writes kana in modified Hepburn romaji: しゃ is sha, ちゃ is cha, じゃ is ja, っ doubles the consonant after
// it (っち is tchi), ん is n (n' before a vowel or y), and ー repeats the vowel before it. を is o. Katakana is read like
// hiragana, and anything that isn't kana is kept as written.
func Romaji(kana string) string {
	rs := []rune(ToHiragana(kana))
	var b strings.Builder
	prev, prevKana := "", rune(0)
	for i := 0; i < len(rs); {
		switch r := rs[i]; r {
		case 'っ':
			next, _ := syllable(rs, i+1)
			switch {
			case strings.HasPrefix(next, "ch"):
				b.WriteByte('t')
			case next != "" && !isVowel(next[0]):
				b.WriteByte(next[0])
			}
			i++
		case 'ん':
			b.WriteByte('n')
			if next, _ := syllable(rs, i+1); next != "" && (isVowel(next[0]) || next[0] == 'y') {
				b.WriteByte('\'')
			}
			prev = "n"
			i++
		case 'ー':
			if v := lastVowel(prev); v != 0 {
				b.WriteByte(v)
			}
			i++
		case 'ゝ':
			b.WriteString(prev)
			i++
		case 'ゞ':
			if voiced, ok := kanaRomaji[prevKana+1]; ok && strings.ContainsRune(voiceable, prevKana) {
				b.WriteString(voiced)
			} else {
				b.WriteString(prev)
			}
			i++
		default:
			syl, n := syllable(rs, i)
			if n == 0 {
				b.WriteRune(r)
				prev, prevKana = "", 0
				i++
				continue
			}
			b.WriteString(syl)
			prev, prevKana = syl, r
			i += n
		}
	}
	return b.String()
}

// RomajiToken is one token of a line, for RomajiLine.
type RomajiToken struct {
	Surface string
	// Reading is the token's kana. It is empty for punctuation, Latin text, and numbers, which are written as they are.
	Reading string
	// Particle marks a particle, so は, へ, and を are read wa, e, and o.
	Particle bool
	// Attach writes the token onto the one before it, with no space: the た in 見た (mita).
	Attach bool
}

// RomajiLine writes a line's tokens in romaji, with a space between words. A token is joined to the one before it when
// it is marked Attach, when the one before ends in っ (待っ and てる are matteru), and when both are Latin text that
// was written without a space between them. Japanese punctuation becomes its Latin form (、 is a comma).
func RomajiLine(tokens []RomajiToken) string {
	type word struct {
		text  string
		kana  bool
		glue  bool
		punct bool
	}
	var words []word
	space, open := false, false
	for _, t := range tokens {
		if strings.TrimFunc(t.Surface, unicode.IsSpace) == "" || t.Surface == "・" {
			space = true
			continue
		}
		var prev *word
		if len(words) > 0 {
			prev = &words[len(words)-1]
		}
		w := word{}
		mark, isMark := punctuation[t.Surface]
		switch {
		case t.Reading != "":
			w = word{text: ToHiragana(t.Reading), kana: true}
			if t.Particle {
				w.text = particleKana(w.text)
			}
			w.glue = t.Attach || (prev != nil && prev.kana && strings.HasSuffix(prev.text, "っ"))
		case isMark:
			w = word{text: mark.text, punct: true, glue: !mark.open}
		default:
			w = word{text: t.Surface, glue: prev != nil && !prev.kana && !prev.punct}
		}
		closing := isMark && !mark.open
		if space && !closing {
			w.glue = false
		}
		if open {
			w.glue = true
		}
		open = isMark && mark.open
		space = false

		if w.glue && w.kana && prev != nil && prev.kana {
			prev.text += w.text
			continue
		}
		words = append(words, w)
	}

	var b strings.Builder
	for i, w := range words {
		if i > 0 && !w.glue {
			b.WriteByte(' ')
		}
		if w.kana {
			b.WriteString(Romaji(w.text))
		} else {
			b.WriteString(w.text)
		}
	}
	return b.String()
}

// particleKana spells a particle the way it sounds, so that は reads wa (including the は at the end of には and では),
// へ reads e, and を reads o.
func particleKana(kana string) string {
	switch {
	case kana == "へ":
		return "え"
	case kana == "を":
		return "お"
	case strings.HasSuffix(kana, "は"):
		return strings.TrimSuffix(kana, "は") + "わ"
	default:
		return kana
	}
}

type mark struct {
	text string
	open bool
}

var punctuation = map[string]mark{
	"、": {text: ","}, "，": {text: ","}, ",": {text: ","},
	"。": {text: "."}, "．": {text: "."}, ".": {text: "."},
	"！": {text: "!"}, "!": {text: "!"},
	"？": {text: "?"}, "?": {text: "?"},
	"…": {text: "..."}, "‥": {text: ".."},
	"～": {text: "~"}, "〜": {text: "~"},
	"「": {text: `"`, open: true}, "」": {text: `"`},
	"『": {text: `"`, open: true}, "』": {text: `"`},
	"（": {text: "(", open: true}, "）": {text: ")"},
	"(": {text: "(", open: true}, ")": {text: ")"},
}

// syllable reads the kana syllable starting at rs[i]: one kana, or two when the second is a small kana that joins it
// (きゃ, ふぁ). It returns the romaji and how many runes it used, or 0 when rs[i] isn't a kana it knows.
func syllable(rs []rune, i int) (string, int) {
	if i >= len(rs) {
		return "", 0
	}
	if i+1 < len(rs) {
		if s, ok := digraphs[string(rs[i:i+2])]; ok {
			return s, 2
		}
		if head, ok := yoon[rs[i]]; ok {
			if v, ok := smallY[rs[i+1]]; ok {
				return head + v, 2
			}
		}
	}
	if s, ok := kanaRomaji[rs[i]]; ok {
		return s, 1
	}
	return "", 0
}

// voiceable are the kana whose voiced form is the next character: か and が, は and ば.
const voiceable = "かきくけこさしすせそたちつてとはひふへほ"

func isVowel(c byte) bool {
	return strings.IndexByte("aeiou", c) >= 0
}

func lastVowel(syl string) byte {
	if syl == "" || !isVowel(syl[len(syl)-1]) {
		return 0
	}
	return syl[len(syl)-1]
}

// yoon is the start of each digraph with a small ゃ, ゅ, or ょ: き and ゃ make kya, し and ゃ make sha.
var yoon = map[rune]string{
	'き': "ky", 'ぎ': "gy", 'し': "sh", 'じ': "j", 'ち': "ch", 'ぢ': "j", 'に': "ny",
	'ひ': "hy", 'び': "by", 'ぴ': "py", 'み': "my", 'り': "ry",
}

var smallY = map[rune]string{'ゃ': "a", 'ゅ': "u", 'ょ': "o"}

// digraphs are the pairs with a small vowel, mostly from loanwords: ファ, ティ, ヴァ, シェ.
var digraphs = map[string]string{
	"ふぁ": "fa", "ふぃ": "fi", "ふぇ": "fe", "ふぉ": "fo", "ふゅ": "fyu",
	"てぃ": "ti", "てゅ": "tyu", "でぃ": "di", "でゅ": "dyu", "とぅ": "tu", "どぅ": "du",
	"うぃ": "wi", "うぇ": "we", "うぉ": "wo", "いぇ": "ye",
	"ゔぁ": "va", "ゔぃ": "vi", "ゔぇ": "ve", "ゔぉ": "vo", "ゔゅ": "vyu",
	"しぇ": "she", "じぇ": "je", "ちぇ": "che",
	"つぁ": "tsa", "つぃ": "tsi", "つぇ": "tse", "つぉ": "tso",
}

var kanaRomaji = map[rune]string{
	'あ': "a", 'い': "i", 'う': "u", 'え': "e", 'お': "o",
	'か': "ka", 'き': "ki", 'く': "ku", 'け': "ke", 'こ': "ko",
	'が': "ga", 'ぎ': "gi", 'ぐ': "gu", 'げ': "ge", 'ご': "go",
	'さ': "sa", 'し': "shi", 'す': "su", 'せ': "se", 'そ': "so",
	'ざ': "za", 'じ': "ji", 'ず': "zu", 'ぜ': "ze", 'ぞ': "zo",
	'た': "ta", 'ち': "chi", 'つ': "tsu", 'て': "te", 'と': "to",
	'だ': "da", 'ぢ': "ji", 'づ': "zu", 'で': "de", 'ど': "do",
	'な': "na", 'に': "ni", 'ぬ': "nu", 'ね': "ne", 'の': "no",
	'は': "ha", 'ひ': "hi", 'ふ': "fu", 'へ': "he", 'ほ': "ho",
	'ば': "ba", 'び': "bi", 'ぶ': "bu", 'べ': "be", 'ぼ': "bo",
	'ぱ': "pa", 'ぴ': "pi", 'ぷ': "pu", 'ぺ': "pe", 'ぽ': "po",
	'ま': "ma", 'み': "mi", 'む': "mu", 'め': "me", 'も': "mo",
	'や': "ya", 'ゆ': "yu", 'よ': "yo",
	'ら': "ra", 'り': "ri", 'る': "ru", 'れ': "re", 'ろ': "ro",
	'わ': "wa", 'ゐ': "i", 'ゑ': "e", 'を': "o",
	'ゔ': "vu",
	'ぁ': "a", 'ぃ': "i", 'ぅ': "u", 'ぇ': "e", 'ぉ': "o",
	'ゃ': "ya", 'ゅ': "yu", 'ょ': "yo", 'ゎ': "wa", 'ゕ': "ka", 'ゖ': "ke",
}
