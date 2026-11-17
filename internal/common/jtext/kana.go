package jtext

import (
	"strings"
	"unicode"
)

// ToHiragana returns s with every katakana letter that has a hiragana twin turned into it: カタカナ becomes かたかな,
// ヴ becomes ゔ, small ッ and ャ become っ and ゃ, and the iteration marks ヽ ヾ become ゝ ゞ. The long vowel mark ー,
// kanji, and characters with no hiragana twin, such as ヷ ヸ ヹ ヺ and the middle dot ・, are left as written.
func ToHiragana(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'ァ' && r <= 'ヶ') || r == 'ヽ' || r == 'ヾ' {
			return r - ('ァ' - 'ぁ')
		}
		return r
	}, s)
}

// IsKana reports whether s is non-empty and made only of hiragana, katakana, and the long vowel mark: text that
// needs no furigana.
func IsKana(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isKana(r) {
			return false
		}
	}
	return true
}

func isKana(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー'
}
