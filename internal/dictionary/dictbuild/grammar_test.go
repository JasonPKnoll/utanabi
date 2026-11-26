package dictbuild_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
)

func readGrammarList(t *testing.T) []dictionary.GrammarPoint {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "grammar.json"))
	if err != nil {
		t.Fatalf("open grammar.json: %v", err)
	}
	defer f.Close()
	list, err := dictbuild.ReadGrammar(f)
	if err != nil {
		t.Fatalf("grammar.json is not valid: %v", err)
	}
	return list
}

func TestGrammarListIsValid(t *testing.T) {
	t.Parallel()

	list := readGrammarList(t)
	if len(list) < 50 || len(list) > 200 {
		t.Errorf("grammar.json has %d points, want 50 to 200", len(list))
	}
	byID := make(map[string]dictionary.GrammarPoint, len(list))
	for _, g := range list {
		byID[g.ID] = g
	}

	// IDs that docs/data-model.md, the schema tests, and the conjugation table already name. They must never go away.
	wantStandard := map[string]string{
		"ga-subject": "", "wa-topic": "", "wo-object": "", "ni-place": "", "ni-time": "", "ni-target": "",
		"ni-purpose": "", "de-place": "", "de-means": "", "he-direction": "", "to-with": "", "to-quote": "",
		"no-possession": "", "no-nominalizer": "", "mo-also": "", "kara-from": "", "kara-because": "",
		"made-until": "", "yori-than": "", "ya-and-so-on": "", "ka-question": "", "ne-agreement": "",
		"yo-emphasis": "", "te-form": "", "teiru": "", "teru": "teiru", "past": "", "negative": "",
		"polite-masu": "", "potential": "", "passive": "", "causative": "", "causative-passive": "",
		"volitional": "", "conditional-ba": "", "conditional-tara": "", "imperative": "", "tai-want": "",
		"te-shimau": "", "chau": "te-shimau", "nu": "negative", "zu": "", "ra-nuki": "potential",
		"na-attributive": "", "te-iku": "", "te-kuru": "", "you-ni": "", "kamoshirenai": "",
		"nakereba-naranai": "", "nakucha": "nakereba-naranai",
	}
	for id, standard := range wantStandard {
		g, ok := byID[id]
		if !ok {
			t.Errorf("grammar.json has no %s", id)
			continue
		}
		if g.StandardID != standard {
			t.Errorf("%s standard_id = %q, want %q", id, g.StandardID, standard)
		}
	}

	for _, g := range list {
		if strings.Contains(g.Explanation+g.Meaning+g.Formation+g.Title, "\u2014") {
			t.Errorf("%s uses an em dash; write it with plain punctuation", g.ID)
		}
	}
}

func TestReadGrammarRejectsBadLists(t *testing.T) {
	t.Parallel()

	const ok = `"kind":"particle","pattern":"が","title":"Subject","meaning":"Marks the subject.",` +
		`"explanation":"Longer text."`
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{name: "empty list", json: `[]`, wantErr: "empty"},
		{name: "not a list", json: `{}`, wantErr: "invalid grammar list"},
		{name: "unknown field", json: `[{"id":"a",` + ok + `,"colour":"red"}]`, wantErr: "colour"},
		{name: "upper case ID", json: `[{"id":"Ga",` + ok + `}]`, wantErr: "ID must be"},
		{name: "ID with a space", json: `[{"id":"ga subject",` + ok + `}]`, wantErr: "ID must be"},
		{name: "ID too long", json: `[{"id":"` + strings.Repeat("a", 65) + `",` + ok + `}]`, wantErr: "ID must be"},
		{name: "duplicate ID", json: `[{"id":"a",` + ok + `},{"id":"a",` + ok + `}]`, wantErr: "used twice"},
		{
			name:    "unknown kind",
			json:    `[{"id":"a","kind":"verb","pattern":"が","title":"t","meaning":"m","explanation":"e"}]`,
			wantErr: "kind must be",
		},
		{
			name:    "missing meaning",
			json:    `[{"id":"a","kind":"particle","pattern":"が","title":"t","explanation":"e"}]`,
			wantErr: "missing its meaning",
		},
		{
			name:    "conjugation without formation",
			json:    `[{"id":"a","kind":"conjugation","pattern":"た","title":"t","meaning":"m","explanation":"e"}]`,
			wantErr: "missing its formation",
		},
		{
			name:    "spaces around a field",
			json:    `[{"id":"a","kind":"particle","pattern":"が","title":" t","meaning":"m","explanation":"e"}]`,
			wantErr: "spaces around it",
		},
		{
			name: "meaning too long",
			json: `[{"id":"a","kind":"particle","pattern":"が","title":"t","meaning":"` + strings.Repeat("あ", 201) +
				`","explanation":"e"}]`,
			wantErr: "longer than 200",
		},
		{
			name:    "unknown standard",
			json:    `[{"id":"a",` + ok + `,"standard_id":"b"}]`,
			wantErr: "names no grammar point",
		},
		{name: "own standard", json: `[{"id":"a",` + ok + `,"standard_id":"a"}]`, wantErr: "its own standard"},
		{
			name: "chain of standards",
			json: `[{"id":"a",` + ok + `},{"id":"b",` + ok + `,"standard_id":"a"},` +
				`{"id":"c",` + ok + `,"standard_id":"b"}]`,
			wantErr: "not itself standard",
		},
		{
			name: "standard of another kind",
			json: `[{"id":"a",` + ok + `},{"id":"b","kind":"pattern","pattern":"p","title":"t","meaning":"m",` +
				`"explanation":"e","standard_id":"a"}]`,
			wantErr: "differ in kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := dictbuild.ReadGrammar(strings.NewReader(tt.json))
			if err == nil {
				t.Fatalf("ReadGrammar(%s) error = nil, want one containing %q", tt.json, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ReadGrammar error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
