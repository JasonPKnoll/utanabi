package songs

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/morph"
)

func searchingTokens() []Token {
	return []Token{
		{Surface: "君", Reading: "きみ", POS: morph.Pronoun}, {Surface: "の", Reading: "の", POS: morph.Particle},
		{Surface: "声", Reading: "こえ", POS: morph.Noun}, {Surface: "を", Reading: "を", POS: morph.Particle},
		{Surface: "探し", Reading: "さがし", POS: morph.Verb}, {Surface: "て", Reading: "て", POS: morph.Particle},
		{Surface: "いる", Reading: "いる", POS: morph.Verb},
	}
}

func searchingAnalyzer() fakeAnalyzer {
	return fakeAnalyzer{tokens: map[string][]Token{"君の声を探している": searchingTokens()}}
}

func TestAddSongAnalysesEachLine(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	got, err := AddSong(t.Context(), repo, searchingAnalyzer(), noMeanings(t), NewSong{Title: "星の歌", Lyrics: "君の声を探している"})
	if err != nil {
		t.Fatalf("AddSong: %v", err)
	}

	if saved := repo.added[0].Lines[0].Tokens; !reflect.DeepEqual(saved, searchingTokens()) {
		t.Errorf("saved tokens = %+v, want %+v", saved, searchingTokens())
	}
	line := got.Lines[0]
	wantRuby := []RubyPart{
		{Text: "君", Reading: "きみ"}, {Text: "の"}, {Text: "声", Reading: "こえ"}, {Text: "を"},
		{Text: "探", Reading: "さが"}, {Text: "している"},
	}
	if !reflect.DeepEqual(line.Ruby, wantRuby) {
		t.Errorf("ruby = %+v, want %+v", line.Ruby, wantRuby)
	}
	if want := "kimi no koe o sagashite iru"; line.Romaji != want {
		t.Errorf("romaji = %q, want %q", line.Romaji, want)
	}
	if line.Text != "君の声を探している" {
		t.Errorf("text = %q, want the line as sung", line.Text)
	}
}

func TestAddSongSavesALineWithoutTokensWhenTheyDontFitIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tokens []Token
	}{
		{name: "a token is missing", tokens: []Token{{Surface: "夜の", Reading: "よるの"}}},
		{name: "a token isn't in the line", tokens: []Token{{Surface: "夜の空"}, {Surface: "で"}}},
		{name: "an empty token", tokens: []Token{{Surface: "夜の空"}, {Surface: ""}}},
		{name: "no tokens", tokens: []Token{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newFakeRepo()
			analyzer := fakeAnalyzer{tokens: map[string][]Token{"夜の空": tt.tokens}}
			got, err := AddSong(t.Context(), repo, analyzer, noMeanings(t), NewSong{Title: "星の歌", Lyrics: "夜の空\n星"})
			if err != nil {
				t.Fatalf("AddSong: %v", err)
			}
			if tokens := repo.added[0].Lines[0].Tokens; tokens != nil {
				t.Errorf("saved tokens %+v, want none", tokens)
			}
			if tokens := repo.added[0].Lines[1].Tokens; len(tokens) != 1 {
				t.Errorf("the other line has %d tokens, want 1", len(tokens))
			}
			line := got.Lines[0]
			if want := []RubyPart{{Text: "夜の空"}}; !reflect.DeepEqual(line.Ruby, want) || line.Romaji != "" {
				t.Errorf("ruby %+v romaji %q, want %+v and no romaji", line.Ruby, line.Romaji, want)
			}
		})
	}
}

func TestAddingASongFailsWhenTheAnalyzerFails(t *testing.T) {
	t.Parallel()

	broken := errors.New("songs test: analyzer broke")
	repo := newFakeRepo()
	analyzer := fakeAnalyzer{err: broken}

	if _, err := AddSong(t.Context(), repo, analyzer, noMeanings(t), NewSong{Title: "星の歌", Lyrics: "星"}); !errors.Is(err, broken) {
		t.Errorf("AddSong err = %v, want %v", err, broken)
	}
	if _, err := AddFromSource(t.Context(), repo, analyzer, noMeanings(t), []LyricsSource{newLRCLIBFake()}, "lrclib", "11"); !errors.Is(err, broken) {
		t.Errorf("AddFromSource err = %v, want %v", err, broken)
	}
	if len(repo.added) != 0 {
		t.Errorf("saved %d songs, want none", len(repo.added))
	}
}

func TestAddFromSourceAnalysesEachLine(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	analyzer := fakeAnalyzer{tokens: map[string][]Token{
		"夜の空に": {
			{Surface: "夜", Reading: "よる", POS: morph.Noun}, {Surface: "の", Reading: "の", POS: morph.Particle},
			{Surface: "空", Reading: "そら", POS: morph.Noun}, {Surface: "に", Reading: "に", POS: morph.Particle},
		},
	}}
	got, err := AddFromSource(t.Context(), repo, analyzer, noMeanings(t), []LyricsSource{newLRCLIBFake()}, "lrclib", "11")
	if err != nil {
		t.Fatalf("AddFromSource: %v", err)
	}
	if len(repo.added[0].Lines[0].Tokens) != 4 {
		t.Errorf("saved tokens = %+v, want 4", repo.added[0].Lines[0].Tokens)
	}
	if want := "yoru no sora ni"; got.Lines[0].Romaji != want {
		t.Errorf("romaji = %q, want %q", got.Lines[0].Romaji, want)
	}
}

func TestGetSongAnalysesLinesSavedWithoutTokens(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Title: "星の歌", Source: SourceUser, Lines: []Line{
		{Position: 0, Text: "君の声を探している"},
		{Position: 1, Text: "星", Tokens: []Token{{Surface: "星", Reading: "ほし", POS: morph.Noun}}},
	}}

	got, err := GetSong(t.Context(), repo, searchingAnalyzer(), noMeanings(t), 1)
	if err != nil {
		t.Fatalf("GetSong: %v", err)
	}
	if repo.tokenSaves != 1 {
		t.Errorf("saved tokens %d times, want once", repo.tokenSaves)
	}
	if tokens := repo.songs[1].Lines[0].Tokens; !reflect.DeepEqual(tokens, searchingTokens()) {
		t.Errorf("stored tokens = %+v, want %+v", tokens, searchingTokens())
	}
	if got.Lines[0].Romaji != "kimi no koe o sagashite iru" || got.Lines[1].Romaji != "hoshi" {
		t.Errorf("romaji = %q, %q, want both lines in romaji", got.Lines[0].Romaji, got.Lines[1].Romaji)
	}

	if _, err := GetSong(t.Context(), repo, searchingAnalyzer(), noMeanings(t), 1); err != nil {
		t.Fatalf("second GetSong: %v", err)
	}
	if repo.tokenSaves != 1 {
		t.Errorf("saved tokens %d times after opening twice, want once", repo.tokenSaves)
	}
}

func TestGetSongFailsWhenTheAnalyzerFails(t *testing.T) {
	t.Parallel()

	broken := errors.New("songs test: analyzer broke")
	repo := newFakeRepo()
	repo.songs[1] = Song{ID: 1, Title: "星の歌", Source: SourceUser, Lines: []Line{{Position: 0, Text: "星"}}}

	if _, err := GetSong(t.Context(), repo, fakeAnalyzer{err: broken}, noMeanings(t), 1); !errors.Is(err, broken) {
		t.Errorf("err = %v, want %v", err, broken)
	}
}

func TestRomajiJoinsWordsByPartOfSpeech(t *testing.T) {
	t.Parallel()

	tok := func(surface, reading string, pos morph.POS) Token {
		return Token{Surface: surface, Reading: reading, POS: pos}
	}
	tests := []struct {
		name   string
		tokens []Token
		want   string
	}{
		{name: "a verb with て and いる", tokens: searchingTokens(), want: "kimi no koe o sagashite iru"},
		{name: "particles", want: "yoru no sora ni hoshi ga hikaru", tokens: []Token{
			tok("夜", "よる", morph.Noun), tok("の", "の", morph.Particle), tok("空", "そら", morph.Noun),
			tok("に", "に", morph.Particle), tok("星", "ほし", morph.Noun), tok("が", "が", morph.Particle),
			tok("光る", "ひかる", morph.Verb),
		}},
		{name: "a stack of conjugations is one word", want: "tabesaserarenakatta", tokens: []Token{
			tok("食べ", "たべ", morph.Verb), tok("させ", "させ", morph.Suffix), tok("られ", "られ", morph.Suffix),
			tok("なかっ", "なかっ", morph.Auxiliary), tok("た", "た", morph.Auxiliary),
		}},
		{name: "volitional", want: "ai ni ikou", tokens: []Token{
			tok("会い", "あい", morph.Verb), tok("に", "に", morph.Particle), tok("行こ", "いこ", morph.Verb),
			tok("う", "う", morph.Auxiliary),
		}},
		{name: "small tsu joins the next token", want: "matteru", tokens: []Token{
			tok("待っ", "まっ", morph.Verb), tok("てる", "てる", morph.Verb),
		}},
		{name: "で after a verb joins it", want: "yonde", tokens: []Token{
			tok("読ん", "よん", morph.Verb), tok("で", "で", morph.Particle),
		}},
		{name: "で after a noun stands apart", want: "sora de", tokens: []Token{
			tok("空", "そら", morph.Noun), tok("で", "で", morph.Particle),
		}},
		{name: "ば after a verb joins it", want: "ikeba", tokens: []Token{
			tok("行け", "いけ", morph.Verb), tok("ば", "ば", morph.Particle),
		}},
		{name: "a copula after a noun stands apart", want: "kimi da", tokens: []Token{
			tok("君", "きみ", morph.Pronoun), tok("だ", "だ", morph.Auxiliary),
		}},
		{name: "a suffix joins its noun", want: "kimitachi wa", tokens: []Token{
			tok("君", "きみ", morph.Pronoun), tok("たち", "たち", morph.Suffix), tok("は", "は", morph.Particle),
		}},
		{name: "は that isn't a particle", want: "hana", tokens: []Token{tok("花", "はな", morph.Noun)}},
		{name: "Latin text and a space", want: "love you kimi", tokens: []Token{
			tok("love", "", morph.Noun), tok(" ", "", morph.Symbol), tok("you", "", morph.Noun),
			tok(" ", "", morph.Symbol), tok("君", "きみ", morph.Pronoun),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := romaji(tt.tokens); got != tt.want {
				t.Errorf("romaji = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRubyJoinsPiecesWithoutReadings(t *testing.T) {
	t.Parallel()

	line := Line{Text: "「お母さん」と", Tokens: []Token{
		{Surface: "「"}, {Surface: "お母さん", Reading: "おかあさん"}, {Surface: "」"}, {Surface: "と", Reading: "と"},
	}}
	want := []RubyPart{{Text: "「お"}, {Text: "母", Reading: "かあ"}, {Text: "さん」と"}}
	if got := ruby(line); !reflect.DeepEqual(got, want) {
		t.Errorf("ruby = %+v, want %+v", got, want)
	}
}
