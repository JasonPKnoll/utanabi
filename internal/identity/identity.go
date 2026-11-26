// Package identity is who a call is for. In server mode the accounts middleware puts the signed-in user in the
// request's context, and Wails hands that context to each bound method; slices read it here, never from a method's
// arguments. On the desktop and phone apps there are no accounts, and every call is the device's own.
package identity

import (
	"context"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
)

// User is a signed-in account.
type User struct {
	ID   int64
	Name string
}

type userKey struct{}

// Into returns ctx carrying the signed-in user.
func Into(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// From returns the signed-in user in ctx, if there is one.
func From(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok && u.ID > 0
}

// Device is the owner of the device's own data: everything on the desktop and phone apps, and whatever a server
// saved before it had accounts. Account IDs start at 1.
const Device int64 = 0

// Owners decides whose data a call reads and writes.
type Owners struct {
	accounts bool
}

// Accounts is for server mode: every call needs a signed-in user, and works on that user's data alone.
func Accounts() Owners {
	return Owners{accounts: true}
}

// OnDevice is for the desktop and phone apps: a call works on the device's data, or a signed-in user's if there is
// one.
func OnDevice() Owners {
	return Owners{}
}

// Of returns the owner for a call, or fault.ErrUnauthorized when accounts are on and no one is signed in. It never
// falls back to the device's data in server mode, so a call that somehow skips the middleware sees nothing.
func (o Owners) Of(ctx context.Context) (int64, error) {
	if u, ok := From(ctx); ok {
		return u.ID, nil
	}
	if o.accounts {
		return 0, fault.ErrUnauthorized
	}
	return Device, nil
}
