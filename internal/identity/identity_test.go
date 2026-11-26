package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

func TestOwners(t *testing.T) {
	t.Parallel()

	signedIn := Into(context.Background(), User{ID: 7, Name: "kaze"})
	tests := []struct {
		name    string
		owners  Owners
		ctx     context.Context
		want    int64
		wantErr error
	}{
		{name: "accounts, signed in", owners: Accounts(), ctx: signedIn, want: 7},
		{name: "accounts, no one signed in", owners: Accounts(), ctx: context.Background(), wantErr: fault.ErrUnauthorized},
		{name: "accounts, a user with no ID", owners: Accounts(), ctx: Into(context.Background(), User{}),
			wantErr: fault.ErrUnauthorized},
		{name: "on the device", owners: OnDevice(), ctx: context.Background(), want: Device},
		{name: "on the device, signed in", owners: OnDevice(), ctx: signedIn, want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.owners.Of(tt.ctx)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Errorf("Of = %d, %v, want %d, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
