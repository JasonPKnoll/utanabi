package jtext

import "strings"

// ToHiragana returns s with every katakana letter that has a hiragana twin turned into it: カラ becomes から, ヴ
// becomes ゔ, and ヽ ヾ become ゝ ゞ. The long vowel mark ー, kanji, and characters with no hiragana twin, such as ヷ and
// the middle dot ・, are left as written.
func ToHiragana(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'ァ' && r <= 'ヶ') || r == 'ヽ' || r == 'ヾ' {
			return r - ('ァ' - 'ぁ')
		}
		return r
	}, s)
}
