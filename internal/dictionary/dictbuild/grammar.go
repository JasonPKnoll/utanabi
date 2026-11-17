package dictbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
)

// ReadGrammar reads and checks the hand-written grammar list: a JSON array of grammar points. It checks everything
// the grammar_points table checks, and more: IDs are unique, every standard_id names a point of the same kind that is
// itself standard, and no text has stray spaces around it.
func ReadGrammar(r io.Reader) ([]dictionary.GrammarPoint, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("dictbuild: read grammar list failed (%v)", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var list []dictionary.GrammarPoint
	if err := dec.Decode(&list); err != nil {
		return nil, fmt.Errorf("dictbuild: invalid grammar list (%v)", err)
	}
	if dec.More() {
		return nil, errors.New("dictbuild: invalid grammar list (more than one JSON value)")
	}
	if err := checkGrammar(list); err != nil {
		return nil, err
	}
	return list, nil
}

func checkGrammar(list []dictionary.GrammarPoint) error {
	if len(list) == 0 {
		return errors.New("dictbuild: the grammar list is empty")
	}
	byID := make(map[string]dictionary.GrammarPoint, len(list))
	for _, g := range list {
		if err := checkGrammarPoint(g); err != nil {
			return err
		}
		if _, dup := byID[g.ID]; dup {
			return fmt.Errorf("dictbuild: grammar point ID is used twice (%s)", g.ID)
		}
		byID[g.ID] = g
	}
	for _, g := range list {
		if g.StandardID == "" {
			continue
		}
		std, ok := byID[g.StandardID]
		switch {
		case !ok:
			return fmt.Errorf("dictbuild: grammar point standard_id names no grammar point (%s to %s)",
				g.ID, g.StandardID)
		case std.ID == g.ID:
			return fmt.Errorf("dictbuild: grammar point is its own standard (%s)", g.ID)
		case std.StandardID != "":
			return fmt.Errorf("dictbuild: grammar point standard is not itself standard (%s to %s)", g.ID, std.ID)
		case std.Kind != g.Kind:
			return fmt.Errorf("dictbuild: grammar point and its standard differ in kind (%s to %s)", g.ID, std.ID)
		}
	}
	return nil
}

func checkGrammarPoint(g dictionary.GrammarPoint) error {
	if !dictionary.ValidGrammarID(g.ID) {
		return fmt.Errorf("dictbuild: grammar point ID must be 1 to 64 characters of a to z, 0 to 9, and - (%q)", g.ID)
	}
	switch g.Kind {
	case dictionary.KindParticle, dictionary.KindConjugation, dictionary.KindPattern:
	default:
		return fmt.Errorf("dictbuild: grammar point kind must be particle, conjugation, or pattern (%s)", g.ID)
	}
	fields := []struct {
		name     string
		value    string
		max      int
		optional bool
	}{
		{name: "pattern", value: g.Pattern, max: 50},
		{name: "title", value: g.Title, max: 80},
		{name: "meaning", value: g.Meaning, max: 200},
		{name: "explanation", value: g.Explanation, max: 2000},
		{name: "formation", value: g.Formation, max: 2000, optional: g.Kind != dictionary.KindConjugation},
	}
	for _, f := range fields {
		switch {
		case f.value == "" && f.optional:
		case f.value == "":
			return fmt.Errorf("dictbuild: grammar point is missing its %s (%s)", f.name, g.ID)
		case strings.TrimSpace(f.value) != f.value:
			return fmt.Errorf("dictbuild: grammar point %s has spaces around it (%s)", f.name, g.ID)
		case utf8.RuneCountInString(f.value) > f.max:
			return fmt.Errorf("dictbuild: grammar point %s is longer than %d characters (%s)", f.name, f.max, g.ID)
		}
	}
	return nil
}
