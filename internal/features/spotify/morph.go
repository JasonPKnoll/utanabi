package spotify

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// Morph is the Romanizer backed by the kagome analyzer in internal/common/morph.
type Morph struct {
	analyzer *morph.Analyzer
}

// NewMorph returns a Romanizer that uses a. The analyzer's dictionary loads on first use, not here.
func NewMorph(a *morph.Analyzer) *Morph {
	return &Morph{analyzer: a}
}

// Romaji writes text in romaji, word by word: 私生活 is shiseikatsu, and 東京事変 is toukyou jihen. Latin text stays as
// written.
func (m *Morph) Romaji(ctx context.Context, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	tokens, err := m.analyzer.Analyze(text)
	if err != nil {
		return "", err
	}
	in := make([]jtext.RomajiToken, len(tokens))
	for i, t := range tokens {
		in[i] = jtext.RomajiToken{Surface: t.Surface, Reading: t.Reading, Particle: t.POS == morph.Particle}
	}
	return jtext.RomajiLine(in), nil
}
