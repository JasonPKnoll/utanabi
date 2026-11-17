package songs

import (
	"context"
	"strings"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// analyzeLines sets each line's tokens. A line whose tokens don't join back into exactly its text keeps none, so it
// is saved without furigana instead of with wrong furigana.
func analyzeLines(ctx context.Context, analyzer Analyzer, lines []Line) error {
	for i := range lines {
		tokens, err := analyzer.Analyze(ctx, lines[i].Text)
		if err != nil {
			return err
		}
		lines[i].Tokens = nil
		if fits(tokens, lines[i].Text) {
			lines[i].Tokens = tokens
		}
	}
	return nil
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

// loadSong reads a song and analyses any of its lines that have no tokens yet, such as songs saved before lines were
// analysed, and saves those tokens. Older songs get furigana the first time they're opened, with no backfill step.
func loadSong(ctx context.Context, repo Repository, analyzer Analyzer, id int64) (Song, error) {
	song, err := repo.SongByID(ctx, id)
	if err != nil {
		return Song{}, err
	}
	var missing []Line
	for _, line := range song.Lines {
		if line.Tokens == nil {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return withReadings(song), nil
	}
	if err := analyzeLines(ctx, analyzer, missing); err != nil {
		return Song{}, err
	}
	if err := repo.SaveTokens(ctx, song.ID, missing); err != nil {
		return Song{}, err
	}
	analysed := make(map[int][]Token, len(missing))
	for _, line := range missing {
		analysed[line.Position] = line.Tokens
	}
	for i := range song.Lines {
		if tokens, ok := analysed[song.Lines[i].Position]; ok {
			song.Lines[i].Tokens = tokens
		}
	}
	return withReadings(song), nil
}

// withReadings works out each line's furigana and romaji from its tokens.
func withReadings(song Song) Song {
	lines := make([]Line, len(song.Lines))
	for i, line := range song.Lines {
		line.Ruby, line.Romaji = ruby(line), romaji(line.Tokens)
		lines[i] = line
	}
	song.Lines = lines
	return song
}

// ruby gives the furigana for each token, and joins the pieces that have none, so 君の声 is 君 (きみ), の, 声 (こえ).
func ruby(line Line) []RubyPart {
	if len(line.Tokens) == 0 {
		return []RubyPart{{Text: line.Text}}
	}
	var parts []RubyPart
	for _, t := range line.Tokens {
		for _, p := range jtext.Ruby(t.Surface, t.Reading) {
			if n := len(parts); n > 0 && p.Reading == "" && parts[n-1].Reading == "" {
				parts[n-1].Text += p.Text
				continue
			}
			parts = append(parts, RubyPart{Text: p.Text, Reading: p.Reading})
		}
	}
	return parts
}

func romaji(tokens []Token) string {
	if len(tokens) == 0 {
		return ""
	}
	in := make([]jtext.RomajiToken, len(tokens))
	for i, t := range tokens {
		in[i] = jtext.RomajiToken{Surface: t.Surface, Reading: t.Reading, Particle: t.POS == morph.Particle,
			Attach: attaches(tokens, i)}
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
