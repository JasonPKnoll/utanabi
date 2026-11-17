package songs

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

// Morph is the Analyzer backed by the kagome analyzer in internal/common/morph.
type Morph struct {
	analyzer *morph.Analyzer
}

// NewMorph returns an Analyzer that uses a. The dictionary loads on the first line analysed, not here.
func NewMorph(a *morph.Analyzer) *Morph {
	return &Morph{analyzer: a}
}

// Analyze splits one line into tokens.
func (m *Morph) Analyze(ctx context.Context, text string) ([]Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	analysed, err := m.analyzer.Analyze(text)
	if err != nil {
		return nil, err
	}
	tokens := make([]Token, len(analysed))
	for i, t := range analysed {
		tokens[i] = Token{Surface: t.Surface, Reading: t.Reading, POS: t.POS, Detail: t.POSDetail,
			Detail2: t.POSDetail2, BaseForm: t.BaseForm, ConjType: t.ConjType, ConjForm: t.ConjForm}
	}
	return tokens, nil
}
