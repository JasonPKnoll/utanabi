//go:build integration

package songs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
	"github.com/JasonPKnoll/utanabi/internal/dictionary"
	"github.com/JasonPKnoll/utanabi/internal/dictionary/dictbuild"
)

func searchingWords() []WordAnalysis {
	return []WordAnalysis{
		{FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "君", LemmaReading: "きみ", EntryID: 1247250,
			Source: AnalysisAuto},
		{FirstToken: 1, LastToken: 1, Kind: WordParticle, Lemma: "の", LemmaReading: "の",
			GrammarPointID: "no-possession", Source: AnalysisAuto},
		{FirstToken: 2, LastToken: 2, Kind: WordContent, Lemma: "声", LemmaReading: "こえ", EntryID: 1380440,
			SenseIndex: 2, Source: AnalysisAuto},
		{FirstToken: 3, LastToken: 3, Kind: WordParticle, Lemma: "を", LemmaReading: "を",
			GrammarPointID: "wo-object", Source: AnalysisAuto},
		{FirstToken: 4, LastToken: 6, Kind: WordContent, Lemma: "探す", LemmaReading: "さがす", ConjClass: "v5s",
			EntryID: 1593670, Steps: []string{"te-form", "teiru"}, Source: AnalysisAuto},
	}
}

func searchingLine() Line {
	return Line{Position: 0, Text: "君の声を探している", Tokens: searchingTokens(), Analysis: searchingWords()}
}

// withoutIDs drops the database IDs and unexported fields, for comparing stored words with the ones saved.
func withoutIDs(words []WordAnalysis) []WordAnalysis {
	out := make([]WordAnalysis, len(words))
	for i, w := range words {
		out[i] = WordAnalysis{FirstToken: w.FirstToken, LastToken: w.LastToken, Kind: w.Kind, Lemma: w.Lemma,
			LemmaReading: w.LemmaReading, ConjClass: w.ConjClass, EntryID: w.EntryID, SenseIndex: w.SenseIndex,
			GrammarPointID: w.GrammarPointID, Steps: w.Steps, Source: w.Source}
	}
	return out
}

func TestSQLiteKeepsEachLinesWords(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	lines := []Line{searchingLine(), {Position: 1, Text: "love", Tokens: []Token{{Surface: "love", POS: morph.Noun}}}}
	added, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: lines})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	got, err := repo.SongByID(t.Context(), added.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(withoutIDs(got.Lines[0].Analysis), searchingWords()) {
		t.Errorf("words = %+v\nwant %+v", withoutIDs(got.Lines[0].Analysis), searchingWords())
	}
	for _, w := range got.Lines[0].Analysis {
		if w.ID == 0 {
			t.Errorf("word %s has no ID", w.Lemma)
		}
	}
	if got.Lines[1].Analysis != nil {
		t.Errorf("a line saved without words has %+v, want nil", got.Lines[1].Analysis)
	}
}

func TestSQLiteAddWithABadWordSavesNothing(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	line := searchingLine()
	line.Analysis[1].GrammarPointID = ""
	if _, err := repo.AddSong(t.Context(), Song{Title: "broken", Source: SourceUser, Lines: []Line{line}}); err == nil {
		t.Fatal("AddSong saved a particle without a grammar point, want an error")
	}
	list, err := repo.Songs(t.Context())
	if err != nil {
		t.Fatalf("Songs: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a failed add left %d songs behind, want none", len(list))
	}
}

func TestSQLiteSaveAnalysisFillsOnlyLinesWithoutWords(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	first := searchingLine()
	first.Analysis = nil
	second := Line{Position: 1, Text: "星", Tokens: []Token{{Surface: "星", Reading: "ほし", POS: morph.Noun}},
		Analysis: []WordAnalysis{{FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "星", Source: AnalysisUser}}}
	song, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: []Line{first, second}})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	update := []Line{searchingLine(), second}
	update[1].Analysis = []WordAnalysis{{FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "ほし"}}
	if err := repo.SaveAnalysis(t.Context(), song.ID, update); err != nil {
		t.Fatalf("SaveAnalysis: %v", err)
	}
	if err := repo.SaveAnalysis(t.Context(), song.ID, update); err != nil {
		t.Fatalf("SaveAnalysis again: %v", err)
	}

	got, err := repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(withoutIDs(got.Lines[0].Analysis), searchingWords()) {
		t.Errorf("line 0 words = %+v, want the saved analysis once", withoutIDs(got.Lines[0].Analysis))
	}
	if words := got.Lines[1].Analysis; len(words) != 1 || words[0].Lemma != "星" || words[0].Source != AnalysisUser {
		t.Errorf("line 1 words = %+v, want your word kept", words)
	}
}

func TestSQLiteSaveMeaningsFillsOnlyAutoWordsWithoutAnEntry(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	line := Line{Position: 0, Text: "星空", Tokens: []Token{
		{Surface: "星", Reading: "ほし", POS: morph.Noun}, {Surface: "空", Reading: "そら", POS: morph.Noun},
	}, Analysis: []WordAnalysis{
		{FirstToken: 0, LastToken: 0, Kind: WordContent, Lemma: "星", Source: AnalysisAuto},
		{FirstToken: 1, LastToken: 1, Kind: WordContent, Lemma: "空", Source: AnalysisUser},
	}}
	song, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: []Line{line}})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	stored, err := repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	words := stored.Lines[0].Analysis
	words[0].EntryID, words[0].SenseIndex = 1376310, 1
	words[1].EntryID = 1245290
	if err := repo.SaveMeanings(t.Context(), words); err != nil {
		t.Fatalf("SaveMeanings: %v", err)
	}
	words[0].EntryID = 999
	if err := repo.SaveMeanings(t.Context(), words[:1]); err != nil {
		t.Fatalf("SaveMeanings again: %v", err)
	}

	got, err := repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	after := got.Lines[0].Analysis
	if after[0].EntryID != 1376310 || after[0].SenseIndex != 1 {
		t.Errorf("星 = entry %d sense %d, want 1376310 sense 1, set once", after[0].EntryID, after[0].SenseIndex)
	}
	if after[1].EntryID != 0 {
		t.Errorf("your word 空 got entry %d, want it left alone", after[1].EntryID)
	}
}

// openFixtureDictionary builds a small dictionary.db from internal/dictionary's test data with the real builder.
func openFixtureDictionary(t *testing.T) *JMdict {
	t.Helper()
	jmdict, err := os.Open(filepath.Join("..", "..", "dictionary", "testdata", "jmdict-eng-common-fixture.json"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer jmdict.Close()
	grammar, err := dictionary.Grammar()
	if err != nil {
		t.Fatalf("Grammar: %v", err)
	}
	path := filepath.Join(t.TempDir(), "dictionary.db")
	if _, err := dictbuild.Build(t.Context(), path, jmdict, grammar, time.Now()); err != nil {
		t.Fatalf("Build: %v", err)
	}
	d, err := dictionary.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	j, err := NewJMdict(d)
	if err != nil {
		t.Fatalf("NewJMdict: %v", err)
	}
	return j
}

func TestSongsGetWordsAndMeaningsWithTheRealAnalyzerAndDictionary(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	analyzer, dict := NewMorph(morph.New()), openFixtureDictionary(t)
	added, err := AddSong(t.Context(), repo, analyzer, dict, NewSong{Title: "星の歌",
		Lyrics: "夜の空に星が光る\n君の声を探している"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}
	if got, want := added.Lines[0].Gloss, "night · 's · sky · in · star · (subject) · shine"; got != want {
		t.Errorf("gloss = %q, want %q", got, want)
	}

	got, err := GetSong(t.Context(), repo, analyzer, dict, added.ID)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	for i := range got.Lines {
		g, a := got.Lines[i], added.Lines[i]
		if !reflect.DeepEqual(g.Words, a.Words) || !reflect.DeepEqual(g.Pieces, a.Pieces) || g.Gloss != a.Gloss {
			t.Errorf("line %d read back as %+v, want what was added, %+v", i, g.Words, a.Words)
		}
	}
	verb := got.Lines[1].Words[4]
	if verb.Lemma != "探す" || verb.EntryID != 1593670 || verb.Gloss != "to search for" ||
		verb.Form != "探す, in the 〜ている form" {
		t.Errorf("探している = %+v", verb)
	}

	meanings, err := LookupWord(t.Context(), dict, verb.Lemma, verb.LemmaReading, verb.EntryID)
	if err != nil {
		t.Fatalf("LookupWord: %v", err)
	}
	if !meanings.Installed || len(meanings.Senses) != 2 || meanings.Senses[0].PartsOfSpeech[0] == "v5s" {
		t.Errorf("LookupWord(探す) = %+v, want two senses with parts of speech in words", meanings)
	}
}

func TestOpeningASongSavedBeforeWordsFindsThem(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	lines := []Line{{Position: 0, Text: "夜の空に星が光る", Tokens: analysedTokens(t, "夜の空に星が光る")}}
	old, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: lines})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	got, err := GetSong(t.Context(), repo, NewMorph(morph.New()), noMeanings(t), old.ID)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if want := "夜 · 's · 空 · in · 星 · (subject) · 光る"; got.Lines[0].Gloss != want {
		t.Errorf("gloss = %q, want %q", got.Lines[0].Gloss, want)
	}
	stored, err := repo.SongByID(t.Context(), old.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if n := len(stored.Lines[0].Analysis); n != 7 {
		t.Errorf("stored %d words after the song was opened, want 7", n)
	}
	if steps := stored.Lines[0].Analysis[6].Steps; steps != nil {
		t.Errorf("光る steps = %v, want none", steps)
	}

	dict := openFixtureDictionary(t)
	again, err := GetSong(t.Context(), repo, NewMorph(morph.New()), dict, old.ID)
	if err != nil {
		t.Fatalf("GetSong with the dictionary: %v", err)
	}
	if want := "night · 's · sky · in · star · (subject) · shine"; again.Lines[0].Gloss != want {
		t.Errorf("gloss once meanings are installed = %q, want %q", again.Lines[0].Gloss, want)
	}
}

// analysedTokens are the tokens of text as the database keeps them, without the analyzer's IPA fields.
func analysedTokens(t *testing.T, text string) []Token {
	t.Helper()
	tokens, err := NewMorph(morph.New()).Analyze(t.Context(), text)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for i, tok := range tokens {
		tokens[i] = Token{Surface: tok.Surface, Reading: tok.Reading, POS: tok.POS}
	}
	return tokens
}
