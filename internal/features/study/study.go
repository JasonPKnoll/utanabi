// Package study is flashcards: a deck of word cards for each song, and the progress you make on them. Review is a
// simple loop. The app shows a card, you type its reading and its meaning, and a right answer adds one to the card's
// streak while a wrong one puts it back to 0. A card counts as learned while its streak is at least the goal (the
// study_streak setting). A word is one card for all the songs it's in, so what you learn in one song counts in all.
package study

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/jtext"
)

// Deck is a song's flashcards, with the song's title: one per word meaning in the song that the dictionary has, in the order they're first
// sung. Goal is how many right answers in a row learn a card. Installed is false when the dictionary of meanings
// isn't installed, and the cards then have no meanings to ask for.
type Deck struct {
	SongID    int64  `json:"songId"`
	Title     string `json:"title"`
	Goal      int    `json:"goal"`
	Installed bool   `json:"installed"`
	Cards     []Card `json:"cards"`
}

// Card is one word with the meaning it has in the song. The front is Label, the word in its dictionary form, and the
// line it's first sung in, where WordText starts at rune WordStart. The answers are Readings, in hiragana (asked
// only when AskReading: a word written only in kana has nothing to read), and Meanings, every English meaning the
// word has; Meaning is the one it has in the song, shown on the back.
type Card struct {
	ID         int64    `json:"id"`
	Label      string   `json:"label"`
	Reading    string   `json:"reading"`
	Readings   []string `json:"readings"`
	AskReading bool     `json:"askReading"`
	Meaning    string   `json:"meaning"`
	Meanings   []string `json:"meanings"`
	Line       string   `json:"line"`
	WordStart  int      `json:"wordStart"`
	WordText   string   `json:"wordText"`
	Progress   Progress `json:"progress"`
}

// Progress is how a card is going: the right answers in a row now, and all the right and wrong ones so far.
type Progress struct {
	Streak  int  `json:"streak"`
	Right   int  `json:"right"`
	Wrong   int  `json:"wrong"`
	Learned bool `json:"learned"`
}

// SongWord is one word of a song that the dictionary has, where it's sung.
type SongWord struct {
	EntryID      int64
	SenseIndex   int
	Lemma        string
	LemmaReading string
	LineID       int64
	LineText     string
	WordStart    int
	WordText     string
}

// StoredCard is a word card as saved, with its progress.
type StoredCard struct {
	ID           int64
	EntryID      int64
	SenseIndex   int
	Label        string
	LabelReading string
	Progress     Progress
}

// Repository keeps the cards and their progress.
type Repository interface {
	// SongWords returns the song's title and its words that have a dictionary meaning, in the order they're sung, or
	// fault.ErrNotFound when there is no such song.
	SongWords(ctx context.Context, songID int64) (string, []SongWord, error)
	// WordCards returns a card for each word, in the same order, saving the ones that don't exist yet.
	WordCards(ctx context.Context, words []SongWord, now time.Time) ([]StoredCard, error)
	// RecordAnswer adds a right or wrong answer to a card's progress, or returns fault.ErrNotFound for no such card.
	RecordAnswer(ctx context.Context, cardID int64, right bool, now time.Time) (Progress, error)
}

// Entry is a dictionary word: its spellings with their readings, and its meanings.
type Entry struct {
	ID     int64
	Forms  []Form
	Senses []Sense
}

// Form is one spelling of an entry and its reading, in hiragana.
type Form struct {
	Text    string
	Reading string
}

// Sense is one meaning of an entry, as English glosses.
type Sense struct {
	Index   int
	Glosses []string
}

// ErrNoMeanings means the dictionary of meanings isn't installed.
var ErrNoMeanings = errors.New("study: meanings aren't installed")

// Dictionary gives the meanings of entries.
type Dictionary interface {
	// Entries returns the entries with the given IDs, leaving out ones it doesn't have, or ErrNoMeanings.
	Entries(ctx context.Context, ids []int64) ([]Entry, error)
}

// SongDeck returns the deck for a song, saving a card for each of its words that doesn't have one yet.
func SongDeck(ctx context.Context, repo Repository, dict Dictionary, songID int64, goal int, now time.Time) (
	Deck, error,
) {
	if songID <= 0 {
		return Deck{}, fault.Invalid("songId", "must be positive")
	}
	title, words, err := repo.SongWords(ctx, songID)
	if err != nil {
		return Deck{}, err
	}
	words = firstOfEach(words)
	deck := Deck{SongID: songID, Title: title, Goal: goal, Installed: true, Cards: []Card{}}
	var stored []StoredCard
	if len(words) > 0 {
		stored, err = repo.WordCards(ctx, words, now)
		if err != nil {
			return Deck{}, err
		}
	}

	ids := make([]int64, len(stored))
	for i, c := range stored {
		ids[i] = c.EntryID
	}
	// Asked even with no cards, so the deck can say whether the dictionary is what's missing.
	found, err := dict.Entries(ctx, ids)
	if errors.Is(err, ErrNoMeanings) {
		deck.Installed = false
	} else if err != nil {
		return Deck{}, err
	}
	byID := make(map[int64]Entry, len(found))
	for _, e := range found {
		byID[e.ID] = e
	}

	for i, c := range stored {
		card := newCard(c, words[i], byID[c.EntryID])
		card.Progress.Learned = card.Progress.Streak >= goal
		deck.Cards = append(deck.Cards, card)
	}
	return deck, nil
}

// Answer records a right or wrong answer to a card and returns its progress now.
func Answer(ctx context.Context, repo Repository, cardID int64, right bool, goal int, now time.Time) (Progress, error) {
	if cardID <= 0 {
		return Progress{}, fault.Invalid("cardId", "must be positive")
	}
	p, err := repo.RecordAnswer(ctx, cardID, right, now)
	if err != nil {
		return Progress{}, err
	}
	p.Learned = p.Streak >= goal
	return p, nil
}

// firstOfEach keeps the first time each word meaning is sung, so a word that comes back in the chorus is one card.
func firstOfEach(words []SongWord) []SongWord {
	type key struct {
		entry int64
		sense int
	}
	seen := map[key]bool{}
	var out []SongWord
	for _, w := range words {
		k := key{w.EntryID, w.SenseIndex}
		if !seen[k] {
			seen[k] = true
			out = append(out, w)
		}
	}
	return out
}

func newCard(c StoredCard, w SongWord, e Entry) Card {
	card := Card{
		ID:        c.ID,
		Label:     c.Label,
		Reading:   c.LabelReading,
		Readings:  []string{},
		Meanings:  []string{},
		Line:      w.LineText,
		WordStart: w.WordStart,
		WordText:  w.WordText,
		Progress:  c.Progress,
	}
	add := func(reading string) {
		reading = jtext.ToHiragana(reading)
		if reading != "" && !slices.Contains(card.Readings, reading) {
			card.Readings = append(card.Readings, reading)
		}
	}
	add(c.LabelReading)
	// Any reading the dictionary gives this spelling is right too: 一日 is いちにち and ついたち.
	for _, f := range e.Forms {
		if f.Text == c.Label {
			add(f.Reading)
		}
	}
	card.AskReading = !jtext.IsKana(c.Label) && len(card.Readings) > 0

	for _, s := range e.Senses {
		for _, g := range s.Glosses {
			if !slices.Contains(card.Meanings, g) {
				card.Meanings = append(card.Meanings, g)
			}
		}
		if s.Index == c.SenseIndex {
			card.Meaning = strings.Join(s.Glosses, "; ")
		}
	}
	return card
}
