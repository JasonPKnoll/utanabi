//go:build integration

package songs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

func analysedLines() []Line {
	return []Line{
		{Position: 0, Stanza: 0, Text: "夜の空、", Tokens: []Token{
			{Surface: "夜", Reading: "よる", POS: morph.Noun}, {Surface: "の", Reading: "の", POS: morph.Particle},
			{Surface: "空", Reading: "そら", POS: morph.Noun}, {Surface: "、", POS: morph.Symbol},
		}},
		{Position: 1, Stanza: 0, Text: "love", Tokens: []Token{{Surface: "love", POS: morph.Noun}}},
		{Position: 2, Stanza: 1, Text: "星が光る"},
	}
}

func TestSQLiteKeepsEachLinesTokens(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	added, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: analysedLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	got, err := repo.SongByID(t.Context(), added.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	if !reflect.DeepEqual(got.Lines, analysedLines()) {
		t.Errorf("lines = %+v, want %+v", got.Lines, analysedLines())
	}

	var noReading, fromAnalyzer int
	err = repo.db.Querier().QueryRowContext(t.Context(),
		`SELECT count(*) FILTER (WHERE reading IS NULL), count(*) FILTER (WHERE reading_source = 'auto')
		 FROM line_tokens`).Scan(&noReading, &fromAnalyzer)
	if err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if noReading != 2 || fromAnalyzer != 5 {
		t.Errorf("tokens without a reading = %d, from the analyzer = %d, want 2 and 5", noReading, fromAnalyzer)
	}
}

func TestSQLiteAddWithABadTokenSavesNothing(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	lines := []Line{{Position: 0, Stanza: 0, Text: "夜", Tokens: []Token{{Surface: "夜", Reading: "ヨル"}}}}
	if _, err := repo.AddSong(t.Context(), Song{Title: "broken", Source: SourceUser, Lines: lines}); err == nil {
		t.Fatal("AddSong saved a katakana reading, want an error")
	}
	list, err := repo.Songs(t.Context())
	if err != nil {
		t.Fatalf("Songs: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a failed add left %d songs behind, want none", len(list))
	}
}

func TestSQLiteSaveTokensFillsOnlyLinesWithoutTokens(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	song, err := repo.AddSong(t.Context(), Song{Title: "星の歌", Source: SourceUser, Lines: analysedLines()})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	stars := []Token{
		{Surface: "星", Reading: "ほし", POS: morph.Noun}, {Surface: "が", Reading: "が", POS: morph.Particle},
		{Surface: "光る", Reading: "ひかる", POS: morph.Verb},
	}
	update := []Line{
		{Position: 0, Text: "夜の空、", Tokens: []Token{{Surface: "夜の空、"}}},
		{Position: 1, Text: "love"},
		{Position: 2, Text: "星が光る", Tokens: stars},
	}
	if err := repo.SaveTokens(t.Context(), song.ID, update); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if err := repo.SaveTokens(t.Context(), song.ID, update); err != nil {
		t.Fatalf("SaveTokens again: %v", err)
	}

	got, err := repo.SongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	want := analysedLines()
	want[2].Tokens = stars
	if !reflect.DeepEqual(got.Lines, want) {
		t.Errorf("lines = %+v, want %+v", got.Lines, want)
	}
}

func TestSQLiteSaveTokensForAMissingLineIsNotFound(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	song := addSong(t, repo, "星の歌")
	lines := []Line{{Position: 9, Text: "星", Tokens: []Token{{Surface: "星", Reading: "ほし"}}}}
	if err := repo.SaveTokens(t.Context(), song.ID, lines); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("err = %v, want fault.ErrNotFound", err)
	}
	if err := repo.SaveTokens(t.Context(), song.ID+100, lines); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("missing song: err = %v, want fault.ErrNotFound", err)
	}
}

func TestSongsAreAnalysedWithTheRealAnalyzer(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	analyzer := NewMorph(morph.New())
	added, err := AddSong(t.Context(), repo, analyzer, NewSong{Title: "星の歌", Lyrics: "夜の空に星が光る\nlove you 君"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	got, err := GetSong(t.Context(), repo, analyzer, added.ID)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	wantRuby := []RubyPart{
		{Text: "夜", Reading: "よる"}, {Text: "の"}, {Text: "空", Reading: "そら"}, {Text: "に"},
		{Text: "星", Reading: "ほし"}, {Text: "が"}, {Text: "光", Reading: "ひか"}, {Text: "る"},
	}
	if !reflect.DeepEqual(got.Lines[0].Ruby, wantRuby) {
		t.Errorf("ruby = %+v, want %+v", got.Lines[0].Ruby, wantRuby)
	}
	if romaji, want := got.Lines[0].Romaji, "yoru no sora ni hoshi ga hikaru"; romaji != want {
		t.Errorf("romaji = %q, want %q", romaji, want)
	}
	if romaji, want := got.Lines[1].Romaji, "love you kimi"; romaji != want {
		t.Errorf("romaji = %q, want %q", romaji, want)
	}
}

func TestOpeningASongSavedBeforeAnalysisAnalysesIt(t *testing.T) {
	t.Parallel()

	repo := newSQLite(t)
	old := addSong(t, repo, "星の歌")

	got, err := GetSong(t.Context(), repo, NewMorph(morph.New()), old.ID)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if romaji, want := got.Lines[2].Romaji, "kimi no koe o sagashite iru"; romaji != want {
		t.Errorf("romaji = %q, want %q", romaji, want)
	}

	stored, err := repo.SongByID(t.Context(), old.ID)
	if err != nil {
		t.Fatalf("SongByID: %v", err)
	}
	for _, line := range stored.Lines {
		if len(line.Tokens) == 0 {
			t.Errorf("line %q has no stored tokens after it was opened", line.Text)
		}
	}
}
