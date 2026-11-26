package accounts

import (
	"context"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/app"
)

// Service is the bound Wails service for admins: making, listing, and revoking invites. Every method checks the
// signed-in user from the request's context is an admin.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService returns the admin service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Invites returns every invite, newest first.
func (s *Service) Invites(ctx context.Context) ([]Invite, error) {
	list, err := ListInvites(ctx, s.repo)
	return list, app.Fail(ctx, err)
}

// CreateInvite makes an invite that makes one account within a week. Its code is shown this once.
func (s *Service) CreateInvite(ctx context.Context) (NewInvite, error) {
	inv, err := MakeInvite(ctx, s.repo, s.now())
	return inv, app.Fail(ctx, err)
}

// RevokeInvite removes an unused invite, so its code no longer works.
func (s *Service) RevokeInvite(ctx context.Context, id int64) error {
	return app.Fail(ctx, RevokeInvite(ctx, s.repo, id))
}
