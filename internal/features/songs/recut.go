package songs

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

const (
	// recutLongest is the longest word, in characters, a recut looks for.
	recutLongest = 10
	// recutShortest is the shortest: a single kana is too often a word of its own somewhere in the dictionary.
	recutShortest = 2
)

// endParticles are the sentence-ending particles a recut takes at the end of a run of kana: うっせぇわ ends in わ.
var endParticles = []string{"わ", "よ", "ね", "ぜ", "ぞ", "さ", "な"}

// recut cuts again a run of kana kagome couldn't read, with the dictionary: at each point, the longest word it has,
// and a sentence-ending particle at the end. kagome reads はぁ？うっせぇうっせぇうっせぇわ as うっせ, a verb うっする that
// doesn't exist, and one unknown noun ぇうっせぇうっせぇわ; cut again, it's うっせぇ (shut up!) three times, and わ.
// The kana token just before the unknown one is cut with it when the first word found runs across them, since that's
// where kagome's mistake starts. A run the dictionary covers less than half of is left as kagome read it.
//
// It runs on the analyzer's tokens before they're stored, so a line analysed again later cuts the same way.
func (m *meanings) recut(ctx context.Context, tokens []Token) ([]Token, error) {
	var out []Token
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if !unknownKana(t) {
			out = append(out, t)
			continue
		}
		// A set phrase the dictionary has around it is joinUnknown's to find: 知ったこっちゃない.
		inPhrase, err := m.phraseAround(ctx, tokens, i)
		if err != nil {
			return nil, err
		}
		if inPhrase {
			out = append(out, t)
			continue
		}
		var prev *Token
		if n := len(out); n > 0 && jtext.IsKana(out[n-1].Surface) && out[n-1].POS != morph.Particle &&
			out[n-1].POS != morph.Auxiliary && out[n-1].POS != morph.Symbol {
			prev = &out[n-1]
		}
		with, err := m.cutKana(ctx, joined(prev, t))
		if err != nil {
			return nil, err
		}
		cut := false
		if prev != nil && len(with) > 0 && utf8.RuneCountInString(with[0].Surface) > utf8.RuneCountInString(prev.Surface) &&
			covered(with) {
			out, cut = append(out[:len(out)-1], with...), true
		} else if alone, err := m.cutKana(ctx, t.Surface); err != nil {
			return nil, err
		} else if covered(alone) && len(alone) > 1 {
			// One word for the whole token cuts nothing: kagome's token stays, for the rules that join it (おサラバする).
			out, cut = append(out, alone...), true
		} else {
			out = append(out, t)
		}
		if cut && i+1 < len(tokens) && tokens[i+1].POS == morph.Suffix && out[len(out)-1].POS != morph.Noun {
			// kagome read what follows as a suffix of its unknown noun; after a word that isn't a noun, it's a noun of
			// its own: くせぇ口 is "stinking mouth", 口 read くち, not the suffix ぐち.
			i++
			noun, err := m.ownNoun(ctx, tokens[i])
			if err != nil {
				return nil, err
			}
			out = append(out, noun)
		}
	}
	return out, nil
}

// ownNoun is t, a suffix, as a noun of its own, read as the dictionary reads it alone.
func (m *meanings) ownNoun(ctx context.Context, t Token) (Token, error) {
	t.POS, t.Detail = morph.Noun, "一般"
	entries, err := m.lookup(ctx, t.Surface, "")
	if err != nil {
		return Token{}, err
	}
	for _, e := range entries {
		for _, f := range e.Forms {
			if f.Text == t.Surface && jtext.IsHiragana(f.Reading) {
				t.Reading = f.Reading
				return t, nil
			}
		}
	}
	return t, nil
}

// unknownKana reports whether t is kana kagome couldn't read: it gives no dictionary form. A lone small vowel or
// long mark (ぇ, ー) counts too, since it's never a word of its own: it's the end of the one before (くせぇ). Katakana
// alone doesn't: it's mostly loanwords and slang, which the dictionary cuts into nonsense (エイデイ, マジヤバ).
func unknownKana(t Token) bool {
	if t.BaseForm != "" || !jtext.IsKana(t.Surface) || !strings.ContainsFunc(t.Surface, isHiragana) {
		return false
	}
	return utf8.RuneCountInString(t.Surface) >= recutShortest || strings.ContainsAny(t.Surface, "ぁぃぅぇぉァィゥェォー")
}

func joined(prev *Token, t Token) string {
	if prev == nil {
		return t.Surface
	}
	return prev.Surface + t.Surface
}

// cutKana cuts text into the longest words the dictionary has, from the start: each a token with the part of speech
// of its first sense. A sentence-ending particle at the very end is a particle; kana between words that no word
// covers stays one unread token.
func (m *meanings) cutKana(ctx context.Context, text string) ([]Token, error) {
	runes := []rune(text)
	var out []Token
	var unread strings.Builder
	flush := func() {
		if unread.Len() > 0 {
			out = append(out, Token{Surface: unread.String(), Reading: jtext.ToHiragana(unread.String()),
				POS: morph.Noun})
			unread.Reset()
		}
	}
	for p := 0; p < len(runes); {
		word, pos, base, err := m.longestWord(ctx, runes[p:])
		if err != nil {
			return nil, err
		}
		if word != "" {
			flush()
			out = append(out, Token{Surface: word, Reading: jtext.ToHiragana(word), POS: pos, BaseForm: base})
			p += utf8.RuneCountInString(word)
			continue
		}
		if rest := string(runes[p:]); slices.Contains(endParticles, rest) {
			flush()
			out = append(out, Token{Surface: rest, Reading: jtext.ToHiragana(rest), POS: morph.Particle,
				Detail: "終助詞", BaseForm: rest})
			break
		}
		unread.WriteRune(runes[p])
		p++
	}
	flush()
	return out, nil
}

// longestWord returns the longest start of runes that's a word the dictionary has, its part of speech, and its
// dictionary form, or "" when none is. An adjective said roughly counts, as its い form: くせぇ is くさい.
func (m *meanings) longestWord(ctx context.Context, runes []rune) (string, morph.POS, string, error) {
	for n := min(len(runes), recutLongest); n >= recutShortest; n-- {
		form := string(runes[:n])
		entries, err := m.lookup(ctx, form, "")
		if err != nil {
			return "", "", "", err
		}
		if len(entries) > 0 && len(entries[0].Senses) > 0 {
			return form, posOf(entries[0].Senses[0].PartsOfSpeech), form, nil
		}
		base, err := m.roughAdjective(ctx, form)
		if err != nil {
			return "", "", "", err
		}
		if base != "" {
			return form, morph.Adjective, base, nil
		}
	}
	return "", "", "", nil
}

// roughAdjective returns the adjective form is said roughly from, its あい or おい turned into ええ (くせぇ, すげえ),
// when the dictionary has it as an adjective, or "". It's the same sound change spokenAdjective reads in kanji (臭え).
func (m *meanings) roughAdjective(ctx context.Context, form string) (string, error) {
	r := []rune(form)
	n := len(r)
	if n < 3 || (r[n-1] != 'ぇ' && r[n-1] != 'え' && r[n-1] != 'ー') {
		return "", nil
	}
	for plain, rough := range eRow {
		if rough != r[n-2] {
			continue
		}
		base := string(r[:n-2]) + string(plain) + "い"
		entries, err := m.lookup(ctx, base, "")
		if err != nil {
			return "", err
		}
		if hasCode(entries, func(c string) bool { return c == "adj-i" }) {
			return base, nil
		}
	}
	return "", nil
}

// posOf is the part of speech of a JMdict sense's codes, as a token has it.
func posOf(codes []string) morph.POS {
	for _, c := range codes {
		switch {
		case c == "int":
			return morph.Interjection
		case c == "prt":
			return morph.Particle
		case strings.HasPrefix(c, "adj-i"):
			return morph.Adjective
		case strings.HasPrefix(c, "adv"):
			return morph.Adverb
		case strings.HasPrefix(c, "v"):
			return morph.Verb
		}
	}
	return morph.Noun
}

// covered reports whether words the dictionary has make up at least half of the tokens' text.
func covered(tokens []Token) bool {
	var known, all int
	for _, t := range tokens {
		n := utf8.RuneCountInString(t.Surface)
		all += n
		if t.BaseForm != "" {
			known += n
		}
	}
	return all > 0 && known*2 >= all
}

func isHiragana(r rune) bool {
	return r >= 'ぁ' && r <= 'ゖ'
}

// phraseAround reports whether the dictionary has a phrase made of token i with the ones around it, up to three
// before and one after: kagome's unknown ゃない is the end of 知ったこっちゃない.
func (m *meanings) phraseAround(ctx context.Context, tokens []Token, i int) (bool, error) {
	for from := max(0, i-3); from <= i; from++ {
		for to := min(len(tokens)-1, i+1); to >= i; to-- {
			if to == from {
				continue
			}
			var text strings.Builder
			for _, t := range tokens[from : to+1] {
				text.WriteString(t.Surface)
			}
			entries, err := m.lookup(ctx, text.String(), "")
			if err != nil {
				return false, err
			}
			if len(entries) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}
