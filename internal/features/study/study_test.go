package study

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

type fakeRepo struct {
	words    []SongWord
	wordsErr error
	saved    []SongWord
	progress map[int64]Progress
	answers  []bool
}

func (f *fakeRepo) SongWords(context.Context, int64) (string, []SongWord, error) {
	return "星の歌", f.words, f.wordsErr
}

func (f *fakeRepo) WordCards(_ context.Context, words []SongWord, _ time.Time) ([]StoredCard, error) {
	f.saved = words
	cards := make([]StoredCard, len(words))
	for i, w := range words {
		id := int64(100 + i)
		cards[i] = StoredCard{ID: id, EntryID: w.EntryID, SenseIndex: w.SenseIndex, Label: w.Lemma,
			LabelReading: w.LemmaReading, Progress: f.progress[id]}
	}
	return cards, nil
}

func (f *fakeRepo) RecordAnswer(_ context.Context, cardID int64, right bool, _ time.Time) (Progress, error) {
	if cardID == 404 {
		return Progress{}, fault.ErrNotFound
	}
	f.answers = append(f.answers, right)
	return Progress{Streak: 3, Right: 3}, nil
}

type fakeDict struct {
	entries []Entry
	err     error
}

func (f fakeDict) Entries(context.Context, []int64) ([]Entry, error) {
	return f.entries, f.err
}

var (
	shine = SongWord{EntryID: 1, Lemma: "光る", LemmaReading: "ひかる", LineID: 10, LineText: "星が光った",
		WordStart: 2, WordText: "光った"}
	who        = SongWord{EntryID: 2, Lemma: "だれ", LemmaReading: "だれ", LineID: 11, LineText: "愛をくれるのはだれ"}
	shineAgain = SongWord{EntryID: 1, Lemma: "光る", LemmaReading: "ひかる", LineID: 12, LineText: "光る"}
	dict       = fakeDict{entries: []Entry{
		{ID: 1, Forms: []Form{{Text: "光る", Reading: "ひかる"}}, Senses: []Sense{
			{Index: 0, Glosses: []string{"to shine", "to glitter"}}, {Index: 1, Glosses: []string{"to stand out"}},
		}},
		{ID: 2, Forms: []Form{{Text: "誰", Reading: "だれ"}, {Text: "だれ", Reading: "だれ"}},
			Senses: []Sense{{Index: 0, Glosses: []string{"who"}}}},
	}}
)

func TestSongDeckIsOneCardPerWordMeaning(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{words: []SongWord{shine, who, shineAgain}, progress: map[int64]Progress{100: {Streak: 3, Right: 4}}}
	deck, err := SongDeck(t.Context(), repo, dict, 7, 3, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("SongDeck: %v", err)
	}
	if len(repo.saved) != 2 || repo.saved[0].LineID != 10 {
		t.Errorf("saved %+v, want 光る from its first line, then だれ", repo.saved)
	}
	want := []Card{
		{ID: 100, Label: "光る", Reading: "ひかる", Readings: []string{"ひかる"}, AskReading: true,
			Meaning: "to shine; to glitter", Meanings: []string{"to shine", "to glitter", "to stand out"},
			Line: "星が光った", WordStart: 2, WordText: "光った",
			Progress: Progress{Streak: 3, Right: 4, Learned: true}},
		{ID: 101, Label: "だれ", Reading: "だれ", Readings: []string{"だれ"}, AskReading: false,
			Meaning: "who", Meanings: []string{"who"}, Line: "愛をくれるのはだれ"},
	}
	if deck.SongID != 7 || deck.Title != "星の歌" || deck.Goal != 3 || !deck.Installed || !reflect.DeepEqual(deck.Cards, want) {
		t.Errorf("deck = %+v\nwant cards %+v", deck, want)
	}
}

func TestSongDeckAcceptsEveryReadingOfTheSpelling(t *testing.T) {
	t.Parallel()

	day := SongWord{EntryID: 3, Lemma: "一日", LemmaReading: "いちにち"}
	d := fakeDict{entries: []Entry{{ID: 3, Forms: []Form{
		{Text: "一日", Reading: "いちにち"}, {Text: "一日", Reading: "ついたち"}, {Text: "1日", Reading: "いちにち"},
	}, Senses: []Sense{{Index: 0, Glosses: []string{"one day"}}}}}}
	deck, err := SongDeck(t.Context(), &fakeRepo{words: []SongWord{day}}, d, 1, 3, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("SongDeck: %v", err)
	}
	if got := deck.Cards[0].Readings; !reflect.DeepEqual(got, []string{"いちにち", "ついたち"}) {
		t.Errorf("readings = %q, want いちにち and ついたち", got)
	}
}

func TestSongDeckWithoutTheDictionary(t *testing.T) {
	t.Parallel()

	deck, err := SongDeck(t.Context(), &fakeRepo{words: []SongWord{shine}}, fakeDict{err: ErrNoMeanings}, 1, 3,
		time.Unix(1, 0))
	if err != nil {
		t.Fatalf("SongDeck: %v", err)
	}
	if deck.Installed || len(deck.Cards) != 1 || len(deck.Cards[0].Meanings) != 0 || !deck.Cards[0].AskReading {
		t.Errorf("deck = %+v, want one card asking only its reading, and not installed", deck)
	}
}

func TestSongDeckOfASongWithNoWords(t *testing.T) {
	t.Parallel()

	deck, err := SongDeck(t.Context(), &fakeRepo{}, dict, 1, 3, time.Unix(1, 0))
	if err != nil || deck.Cards == nil || len(deck.Cards) != 0 || !deck.Installed {
		t.Errorf("deck = %+v, %v, want an empty list of cards", deck, err)
	}
	// With no dictionary, no word has a meaning, so the deck is empty and says why.
	deck, err = SongDeck(t.Context(), &fakeRepo{}, fakeDict{err: ErrNoMeanings}, 1, 3, time.Unix(1, 0))
	if err != nil || len(deck.Cards) != 0 || deck.Installed {
		t.Errorf("deck without the dictionary = %+v, %v, want empty and not installed", deck, err)
	}
}

func TestSongDeckFailures(t *testing.T) {
	t.Parallel()

	var invalid *fault.InvalidError
	if _, err := SongDeck(t.Context(), &fakeRepo{}, dict, 0, 3, time.Unix(1, 0)); !errors.As(err, &invalid) {
		t.Errorf("SongDeck(0) error = %v, want invalid songId", err)
	}
	missing := &fakeRepo{wordsErr: fault.ErrNotFound}
	if _, err := SongDeck(t.Context(), missing, dict, 9, 3, time.Unix(1, 0)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("SongDeck(missing song) error = %v, want not found", err)
	}
	broken := fakeDict{err: errors.New("disk on fire")}
	if _, err := SongDeck(t.Context(), &fakeRepo{words: []SongWord{shine}}, broken, 1, 3, time.Unix(1, 0)); err == nil {
		t.Error("SongDeck with a failing dictionary succeeded")
	}
}

func TestAnswer(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	p, err := Answer(t.Context(), repo, 100, true, 3, time.Unix(1, 0))
	if err != nil || !p.Learned || !reflect.DeepEqual(repo.answers, []bool{true}) {
		t.Errorf("Answer = %+v, %v, answers %v; want learned at a streak of 3", p, err, repo.answers)
	}
	if p, _ := Answer(t.Context(), repo, 100, true, 4, time.Unix(1, 0)); p.Learned {
		t.Error("a streak of 3 counted as learned with a goal of 4")
	}
	var invalid *fault.InvalidError
	if _, err := Answer(t.Context(), repo, -1, true, 3, time.Unix(1, 0)); !errors.As(err, &invalid) {
		t.Errorf("Answer(-1) error = %v, want invalid cardId", err)
	}
	if _, err := Answer(t.Context(), repo, 404, false, 3, time.Unix(1, 0)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("Answer(missing card) error = %v, want not found", err)
	}
}
