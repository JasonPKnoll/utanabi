package jtext

// RubyPart is a piece of written text with the reading shown over it. Reading is empty for kana and anything else
// that needs no furigana.
type RubyPart struct {
	Text    string
	Reading string
}

// Ruby lines a word's reading up with how it is written, so furigana sits only over the kanji: 光る read ひかる gives
// 光 (ひか) and る, and お母さん read おかあさん gives お, 母 (かあ), and さん. Kana in the word is found in the
// reading and left bare. When the two can't be lined up, the whole reading goes over the whole word. A word with no
// reading, or written only in kana, gets no furigana.
func Ruby(surface, reading string) []RubyPart {
	if surface == "" {
		return nil
	}
	reading = ToHiragana(reading)
	if reading == "" || IsKana(surface) {
		return []RubyPart{{Text: surface}}
	}
	if parts, ok := align(splitRuns(surface), []rune(reading)); ok {
		return parts
	}
	return []RubyPart{{Text: surface, Reading: reading}}
}

type run struct {
	text string
	kana bool
}

// splitRuns cuts s into alternating runs of kana and of everything else (kanji, 々, digits, Latin letters).
func splitRuns(s string) []run {
	var runs []run
	for _, r := range s {
		kana := isKana(r)
		if n := len(runs); n > 0 && runs[n-1].kana == kana {
			runs[n-1].text += string(r)
			continue
		}
		runs = append(runs, run{text: string(r), kana: kana})
	}
	return runs
}

// align matches each kana run literally in the reading and gives each other run the reading between, trying the
// shortest reading first. It reports false when no split uses up exactly the whole reading.
func align(runs []run, reading []rune) ([]RubyPart, bool) {
	if len(runs) == 0 {
		return nil, len(reading) == 0
	}
	first := runs[0]
	if first.kana {
		kana := []rune(ToHiragana(first.text))
		if len(kana) > len(reading) || string(reading[:len(kana)]) != string(kana) {
			return nil, false
		}
		rest, ok := align(runs[1:], reading[len(kana):])
		if !ok {
			return nil, false
		}
		return append([]RubyPart{{Text: first.text}}, rest...), true
	}
	for n := 1; n <= len(reading); n++ {
		if rest, ok := align(runs[1:], reading[n:]); ok {
			return append([]RubyPart{{Text: first.text, Reading: string(reading[:n])}}, rest...), true
		}
	}
	return nil, false
}
