// Package jtext normalizes Japanese text before it is stored, so the same word is always the same bytes.
package jtext

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize returns s in Unicode NFC, with half-width katakana (ｶﾀｶﾅ) turned into full-width (カタカナ) and Windows
// line endings turned into \n. Everything else, including full-width Latin letters, is left as written.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.ContainsFunc(s, isHalfwidthKana) {
		var b strings.Builder
		b.Grow(len(s))
		for _, r := range s {
			if isHalfwidthKana(r) {
				// NFKC maps one half-width kana to its full-width form; a separate ﾞ or ﾟ becomes a combining mark,
				// which the NFC below joins with the kana before it (ｶﾞ → ガ).
				b.WriteString(norm.NFKC.String(string(r)))
				continue
			}
			b.WriteRune(r)
		}
		s = b.String()
	}
	return norm.NFC.String(s)
}

// IsHiragana reports whether s is non-empty and made only of hiragana, the long vowel mark ー, and the iteration marks
// ゝ and ゞ: the characters a stored reading may use.
func IsHiragana(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.Is(unicode.Hiragana, r) && r != 'ー' {
			return false
		}
	}
	return true
}

func isHalfwidthKana(r rune) bool {
	return r >= '｡' && r <= 'ﾟ'
}
