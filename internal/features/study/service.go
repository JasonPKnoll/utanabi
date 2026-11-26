package study

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
)

// Service is the bound Wails service for flashcards. Every exported method here is callable by the frontend.
type Service struct {
	repo Repository
	dict Dictionary
	goal int
	now  func() time.Time
}

// NewService returns the study service. goal is how many right answers in a row learn a card.
func NewService(repo Repository, dict Dictionary, goal int) *Service {
	return &Service{repo: repo, dict: dict, goal: goal, now: time.Now}
}

// Deck returns a song's flashcards with your progress on each, making cards for words that don't have one yet.
func (s *Service) Deck(ctx context.Context, songID int64) (Deck, error) {
	deck, err := SongDeck(ctx, s.repo, s.dict, songID, s.goal, s.now())
	return deck, app.Fail(ctx, err)
}

// Answer records whether you answered a card right, and returns its progress now.
func (s *Service) Answer(ctx context.Context, cardID int64, right bool) (Progress, error) {
	p, err := Answer(ctx, s.repo, cardID, right, s.goal, s.now())
	return p, app.Fail(ctx, err)
}
