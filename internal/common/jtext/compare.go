package jtext

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Plain is text for comparing: NFKC, lower case, letters and digits only, so "Shinunoga E-Wa" is "shinunogaewa".
func Plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, norm.NFKC.String(s))
}

// FoldRomaji is romaji for comparing: Plain, with no accents, and long vowels written one way, since romaji spells
// them many ways: Tōkyō, Toukyou, and Tokyo are all tokyo.
func FoldRomaji(romaji string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(romaji) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	var out []rune
	for _, r := range Plain(b.String()) {
		if n := len(out); n > 0 && slices.Contains([]rune("aeiou"), r) && (out[n-1] == r || (r == 'u' && out[n-1] == 'o')) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
