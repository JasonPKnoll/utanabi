package songs

import (
	"context"
	"strings"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// analyzeLines sets each line's tokens and words, with the words' meanings when the dictionary is installed. A line
// whose tokens don't join back into exactly its text keeps none, and no words, so it is saved without furigana
// instead of with wrong furigana.
func analyzeLines(ctx context.Context, analyzer Analyzer, dict Dictionary, lines []Line) error {
	m := newMeanings(dict)
	for i := range lines {
		if err := analyzeLine(ctx, analyzer, m, &lines[i]); err != nil {
			return err
		}
	}
	return nil
}

func analyzeLine(ctx context.Context, analyzer Analyzer, m *meanings, line *Line) error {
	tokens, err := analyzer.Analyze(ctx, line.Text)
	if err != nil {
		return err
	}
	line.Tokens, line.Analysis = nil, nil
	if !fits(tokens, line.Text) {
		return nil
	}
	line.Tokens = tokens
	return m.analyzeWords(ctx, line)
}

func fits(tokens []Token, text string) bool {
	if len(tokens) == 0 {
		return false
	}
	var b strings.Builder
	for _, t := range tokens {
		if t.Surface == "" {
			return false
		}
		b.WriteString(t.Surface)
	}
	return b.String() == text
}

// loadSong reads a song and finishes the analysis of any of its lines that need it, then saves what it found:
// tokens for lines that have none, such as songs saved before lines were analysed; words for lines that have tokens
// but no words, such as songs saved before words were found; and meanings for words found while the dictionary
// wasn't installed. Older songs catch up the first time they're opened, with no backfill step.
func loadSong(ctx context.Context, repo Repository, analyzer Analyzer, dict Dictionary, id int64) (Song, error) {
	song, err := repo.SongByID(ctx, id)
	if err != nil {
		return Song{}, err
	}
	m := newMeanings(dict)
	var analysed []Line
	for i := range song.Lines {
		line := &song.Lines[i]
		switch {
		case line.Tokens == nil:
			if err := analyzeLine(ctx, analyzer, m, line); err != nil {
				return Song{}, err
			}
			if line.Tokens != nil {
				analysed = append(analysed, *line)
			}
		case line.Analysis == nil && hasWords(line.Tokens):
			if err := reanalyze(ctx, analyzer, m, line); err != nil {
				return Song{}, err
			}
			if line.Analysis != nil {
				analysed = append(analysed, *line)
			}
		}
	}
	if len(analysed) > 0 {
		if err := repo.SaveAnalysis(ctx, song.ID, analysed); err != nil {
			return Song{}, err
		}
	}
	if err := fillMeanings(ctx, repo, m, song.Lines); err != nil {
		return Song{}, err
	}
	return present(ctx, dict, song)
}

func hasWords(tokens []Token) bool {
	for _, t := range tokens {
		if !outsideWords(t) {
			return true
		}
	}
	return false
}

// reanalyze finds the words of a line that has stored tokens but no words. Stored tokens don't keep the analyzer's IPA
// fields, so the line is analysed again and the fields are copied over, keeping the stored readings, which may be
// corrections. When the new tokens don't match the stored ones, the line is left without words.
func reanalyze(ctx context.Context, analyzer Analyzer, m *meanings, line *Line) error {
	fresh, err := analyzer.Analyze(ctx, line.Text)
	if err != nil {
		return err
	}
	if len(fresh) != len(line.Tokens) {
		return nil
	}
	merged := make([]Token, len(fresh))
	for i, t := range fresh {
		stored := line.Tokens[i]
		if t.Surface != stored.Surface {
			return nil
		}
		t.Reading, t.POS = stored.Reading, stored.POS
		merged[i] = t
	}
	line.Tokens = merged
	return m.analyzeWords(ctx, line)
}

// fillMeanings looks up the stored words that have no dictionary entry, which happens when they were found while the
// dictionary wasn't installed, and saves the meanings it finds. Words you corrected are left alone.
func fillMeanings(ctx context.Context, repo Repository, m *meanings, lines []Line) error {
	var found []WordAnalysis
	for _, line := range lines {
		for i := range line.Analysis {
			w := &line.Analysis[i]
			if w.ID == 0 || w.EntryID != 0 || w.Kind != WordContent || w.Source != AnalysisAuto {
				continue
			}
			ok, err := m.resolve(ctx, w, wordCategory(*w, line.Tokens))
			if err != nil {
				return err
			}
			if ok {
				found = append(found, *w)
			}
			if m.missing {
				return nil
			}
		}
	}
	if len(found) == 0 {
		return nil
	}
	return repo.SaveMeanings(ctx, found)
}

// rubyOf gives the furigana for each token, and joins the pieces that have none, so 君の声 is 君 (きみ), の, 声 (こえ).
// A word with a sung reading gets that reading over it instead (see markSung).
func rubyOf(tokens []Token) []RubyPart {
	var parts []RubyPart
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		var pieces []RubyPart
		if t.sung != "" {
			end := min(t.sungEnd, len(tokens)-1)
			pieces = sungRuby(surfaceOf(tokens[i:end+1]), t.sung)
			i = end
		} else {
			for _, p := range jtext.Ruby(t.Surface, t.Reading) {
				pieces = append(pieces, RubyPart{Text: p.Text, Reading: p.Reading})
			}
		}
		for _, p := range pieces {
			if n := len(parts); n > 0 && p.Reading == "" && parts[n-1].Reading == "" {
				parts[n-1].Text += p.Text
				continue
			}
			parts = append(parts, p)
		}
	}
	return parts
}

// ruby gives a line's furigana: one piece with no reading when the line has no tokens.
func ruby(line Line) []RubyPart {
	if len(line.Tokens) == 0 {
		return []RubyPart{{Text: line.Text}}
	}
	return rubyOf(line.Tokens)
}

func romaji(tokens []Token) string {
	if len(tokens) == 0 {
		return ""
	}
	in := make([]jtext.RomajiToken, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.sung != "" {
			// A word with a sung reading is written as it's sung, as one word: 羅針盤 sung コンパス is konpasu.
			end := min(t.sungEnd, len(tokens)-1)
			in = append(in, jtext.RomajiToken{Surface: surfaceOf(tokens[i : end+1]), Reading: jtext.ToHiragana(t.sung),
				Attach: attaches(tokens, i)})
			i = end
			continue
		}
		in = append(in, jtext.RomajiToken{Surface: t.Surface, Reading: t.Reading, Particle: t.POS == morph.Particle,
			Attach: attaches(tokens, i)})
	}
	return jtext.RomajiLine(in)
}

// attaches reports whether token i is written onto the word before it in romaji. Suffixes always are (2nin,
// kimitachi). Auxiliaries are when they follow a verb, adjective, or another auxiliary, so a conjugated verb stays one
// word (tabesaserarenakatta), but a copula after a noun stands apart (kimi da). The particles て, で, and ば are when
// they follow one of those too (sagashite, yonde, ikeba).
func attaches(tokens []Token, i int) bool {
	if i == 0 {
		return false
	}
	afterConjugated := false
	switch tokens[i-1].POS {
	case morph.Verb, morph.Adjective, morph.Auxiliary, morph.Suffix:
		afterConjugated = true
	}
	switch tokens[i].POS {
	case morph.Suffix:
		return true
	case morph.Auxiliary:
		return afterConjugated
	case morph.Particle:
		switch tokens[i].Reading {
		case "て", "で", "ば":
			return afterConjugated
		}
	}
	return false
}
