// Package accounts is sign-in for server mode, with passkeys. A passkey is a key pair: the private half stays on the
// user's device or in their password manager, and the server keeps only the public half, so there's no password to
// steal or guess. Signing in proves the device holds the private half by signing a one-time challenge (WebAuthn).
// A signed-in browser holds a random session token in a cookie, and the server keeps only its hash.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

// SessionLife is how long a sign-in lasts before the user signs in again.
const SessionLife = 30 * 24 * time.Hour

// InviteLife is how long an invite works before it runs out.
const InviteLife = 7 * 24 * time.Hour

// Invite is an invite as an admin sees it: never its code, which is shown only once, when it's made.
type Invite struct {
	ID        int64     `json:"id"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	UsedBy    string    `json:"usedBy"`
	UsedAt    time.Time `json:"usedAt"`
}

// NewInvite is an invite just made, with the code to share. The server keeps only the code's hash.
type NewInvite struct {
	Code   string `json:"code"`
	Invite Invite `json:"invite"`
}

// Account is a user and their passkeys. Handle is the random user handle their passkeys are made for.
type Account struct {
	ID       int64
	Handle   []byte
	Name     string
	Admin    bool
	Passkeys []Passkey
}

// Passkey is the server's half of a passkey: the public key, and what the authenticator said about itself.
type Passkey struct {
	ID              []byte
	PublicKey       []byte
	AttestationType string
	Transports      []string
	AAGUID          []byte
	SignCount       uint32
	BackupEligible  bool
	BackupState     bool
}

// Site is where a passkey is used: the relying party ID (the host) and the page origin, which the browser and the
// server both check, so a passkey can only ever sign in to the site it was made on.
type Site struct {
	ID     string
	Origin string
}

// Repository keeps users, passkeys, and sessions.
type Repository interface {
	// CreateAccount saves a new user with their first passkey, or returns fault.ErrConflict if the name is taken.
	// With an invite, it uses the invite up in the same transaction, or returns ErrInviteGone if it was just used or
	// has run out.
	CreateAccount(ctx context.Context, name string, handle []byte, key Passkey, invite []byte, now time.Time) (
		Account, error,
	)
	// InviteUsable reports whether an invite with this code hash is unused and hasn't run out.
	InviteUsable(ctx context.Context, codeHash []byte, now time.Time) (bool, error)
	// Invites returns every invite, newest first.
	Invites(ctx context.Context) ([]Invite, error)
	// CreateInvite saves an invite by its code's hash and returns it.
	CreateInvite(ctx context.Context, codeHash []byte, createdBy int64, now, expires time.Time) (Invite, error)
	// DeleteInvite removes an unused invite, or returns fault.ErrNotFound when there's no unused invite by that ID.
	DeleteInvite(ctx context.Context, id int64) error
	// AccountByHandle returns the user a passkey's user handle belongs to, or fault.ErrNotFound.
	AccountByHandle(ctx context.Context, handle []byte) (Account, error)
	// AccountByID returns a user with their passkeys, or fault.ErrNotFound.
	AccountByID(ctx context.Context, id int64) (Account, error)
	// AddPasskey saves another passkey for a user, or returns fault.ErrConflict if it's already saved.
	AddPasskey(ctx context.Context, userID int64, key Passkey, now time.Time) error
	// UsedPasskey records a sign-in with a passkey: its new sign count and backup state.
	UsedPasskey(ctx context.Context, keyID []byte, signCount uint32, backupState bool, now time.Time) error
	// CreateSession saves a session by its token's hash, and clears sessions that have run out.
	CreateSession(ctx context.Context, tokenHash []byte, userID int64, now, expires time.Time) error
	// SessionUser returns the user of a session that hasn't run out, or fault.ErrNotFound.
	SessionUser(ctx context.Context, tokenHash []byte, now time.Time) (identity.User, error)
	// DeleteSession ends a session. Ending one that doesn't exist is not an error.
	DeleteSession(ctx context.Context, tokenHash []byte) error
}

// Ceremonies runs the WebAuthn ceremonies. Options are what the browser passes to navigator.credentials, and state is
// what the server keeps between the two halves of a ceremony: above all, its one-time challenge.
type Ceremonies interface {
	BeginRegistration(site Site, a Account) (options json.RawMessage, state []byte, err error)
	// FinishRegistration checks the browser's response against state and returns the new passkey, or
	// ErrPasskeyRejected.
	FinishRegistration(site Site, a Account, state, response []byte) (Passkey, error)
	// BeginSignIn starts a sign-in. A conditional one waits quietly for the person to pick a passkey from their
	// browser's autofill, rather than opening a prompt.
	BeginSignIn(site Site, conditional bool) (options json.RawMessage, state []byte, err error)
	// FinishSignIn checks the browser's response against state, finding the account by the user handle the
	// passkey carries, and returns the account and the passkey as it is now; or ErrPasskeyRejected.
	FinishSignIn(site Site, state, response []byte, find func(handle []byte) (Account, error)) (Account, Passkey, error)
}

// ErrPasskeyRejected means a passkey response didn't check out: a wrong or reused challenge, another site, a bad
// signature, a passkey that isn't this account's, or one that looks copied.
var ErrPasskeyRejected = fault.Invalid("passkey", "couldn't be checked; please try again")

// ErrSignupClosed means the server takes no new accounts, because it has no sign-up code.
var ErrSignupClosed = fault.Invalid("signup", "is closed on this server")

// ErrWrongCode means a sign-up code is neither the server's code nor a usable invite.
var ErrWrongCode = fault.Invalid("code", "isn't right, or the invite has been used or has run out")

// ErrInviteGone means an invite was used by someone else, or ran out, between checking it and making the account.
var ErrInviteGone = fault.Invalid("code", "was just used or has run out; ask for a new invite")

const (
	minNameLen = 3
	maxNameLen = 32
	handleLen  = 32
	tokenLen   = 32
)

// Name checks a username and returns it as stored: trimmed and lowercase. A username is 3 to 32 letters, digits,
// underscores, and hyphens.
func Name(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if n := utf8.RuneCountInString(name); n < minNameLen || n > maxNameLen {
		return "", fault.Invalid("name", "must be 3 to 32 characters")
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "", fault.Invalid("name", "can only use letters, numbers, _ and -")
		}
	}
	return name, nil
}

// inviteAlphabet leaves out letters and digits that look alike (0 and o, 1 and l, i), so an invite read aloud or typed
// on a phone comes out right.
const inviteAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewInviteCode returns a random invite code, four groups of four (about 79 bits), and the hash the server keeps.
func NewInviteCode() (code string, hash []byte, err error) {
	// Bytes past the last whole run of the alphabet are thrown away, so every letter is as likely as every other.
	limit := byte(256 - 256%len(inviteAlphabet))
	var s strings.Builder
	for n := 0; n < 16; {
		b, err := random(32)
		if err != nil {
			return "", nil, err
		}
		for _, v := range b {
			if v >= limit || n == 16 {
				continue
			}
			if n > 0 && n%4 == 0 {
				s.WriteByte('-')
			}
			s.WriteByte(inviteAlphabet[int(v)%len(inviteAlphabet)])
			n++
		}
	}
	code = s.String()
	return code, InviteHash(code), nil
}

// InviteHash is the SHA-256 of an invite code as typed: trimmed and lowercase, since the code has no capitals.
func InviteHash(code string) []byte {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(code))))
	return sum[:]
}

// CheckCode checks a sign-up code: the server's own code, which any number of accounts can use, or an invite, which
// makes one. It returns the invite's hash, or nil for the server's code.
func CheckCode(ctx context.Context, repo Repository, code string, serverCode redact.Secret, now time.Time) (
	[]byte, error,
) {
	if !serverCode.IsZero() && CheckSignupCode(code, serverCode) == nil {
		return nil, nil
	}
	hash := InviteHash(code)
	ok, err := repo.InviteUsable(ctx, hash, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrWrongCode
	}
	return hash, nil
}

// CheckSignupCode reports whether code is the server's sign-up code, in constant time, ignoring spaces around it (a
// phone's keyboard adds one after a suggestion). With no code set, sign-up is closed and nothing matches.
func CheckSignupCode(code string, want redact.Secret) error {
	if want.IsZero() {
		return ErrSignupClosed
	}
	got, expected := sha256.Sum256([]byte(strings.TrimSpace(code))), sha256.Sum256([]byte(want.Reveal()))
	if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
		return fault.Invalid("code", "isn't right")
	}
	return nil
}

// NewHandle returns a random user handle for a new account.
func NewHandle() ([]byte, error) {
	return random(handleLen)
}

// NewSessionToken returns a random session token for the cookie, and the hash the server keeps.
func NewSessionToken() (token string, hash []byte, err error) {
	b, err := random(tokenLen)
	if err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, TokenHash(token), nil
}

// TokenHash is the SHA-256 of a session token, which is what the server stores and looks sessions up by.
func TokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func random(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// SignUp is the second half of making an account: it checks the new passkey and saves the account with it, using up
// the invite it was begun with, if any.
func SignUp(ctx context.Context, repo Repository, cer Ceremonies, site Site, draft Account, invite []byte,
	state, response []byte, now time.Time,
) (Account, error) {
	key, err := cer.FinishRegistration(site, draft, state, response)
	if err != nil {
		return Account{}, err
	}
	return repo.CreateAccount(ctx, draft.Name, draft.Handle, key, invite, now)
}

// requireAdmin returns the signed-in user if they're an admin.
func requireAdmin(ctx context.Context) (identity.User, error) {
	u, ok := identity.From(ctx)
	if !ok {
		return identity.User{}, fault.ErrUnauthorized
	}
	if !u.Admin {
		return identity.User{}, fault.ErrForbidden
	}
	return u, nil
}

// ListInvites returns every invite, for an admin.
func ListInvites(ctx context.Context, repo Repository) ([]Invite, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return repo.Invites(ctx)
}

// MakeInvite makes an invite, for an admin, and returns its code: the only time the code is ever shown.
func MakeInvite(ctx context.Context, repo Repository, now time.Time) (NewInvite, error) {
	u, err := requireAdmin(ctx)
	if err != nil {
		return NewInvite{}, err
	}
	code, hash, err := NewInviteCode()
	if err != nil {
		return NewInvite{}, err
	}
	inv, err := repo.CreateInvite(ctx, hash, u.ID, now, now.Add(InviteLife))
	if err != nil {
		return NewInvite{}, err
	}
	return NewInvite{Code: code, Invite: inv}, nil
}

// RevokeInvite removes an unused invite, for an admin, so its code no longer works.
func RevokeInvite(ctx context.Context, repo Repository, id int64) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	if id <= 0 {
		return fault.Invalid("id", "must be positive")
	}
	return repo.DeleteInvite(ctx, id)
}

// SignIn is the second half of signing in: it checks the passkey's signature, records its use, and returns whose it
// is.
func SignIn(ctx context.Context, repo Repository, cer Ceremonies, site Site, state, response []byte, now time.Time) (
	Account, error,
) {
	find := func(handle []byte) (Account, error) { return repo.AccountByHandle(ctx, handle) }
	account, key, err := cer.FinishSignIn(site, state, response, find)
	if err != nil {
		return Account{}, err
	}
	if err := repo.UsedPasskey(ctx, key.ID, key.SignCount, key.BackupState, now); err != nil {
		return Account{}, err
	}
	return account, nil
}

// AddPasskey is the second half of adding a passkey to a signed-in account, so losing one device doesn't lock its
// owner out.
func AddPasskey(ctx context.Context, repo Repository, cer Ceremonies, site Site, a Account, state, response []byte,
	now time.Time,
) error {
	key, err := cer.FinishRegistration(site, a, state, response)
	if err != nil {
		return err
	}
	return repo.AddPasskey(ctx, a.ID, key, now)
}
