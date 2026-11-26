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

// Account is a user and their passkeys. Handle is the random user handle their passkeys are made for.
type Account struct {
	ID       int64
	Handle   []byte
	Name     string
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
	CreateAccount(ctx context.Context, name string, handle []byte, key Passkey, now time.Time) (Account, error)
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
	BeginSignIn(site Site) (options json.RawMessage, state []byte, err error)
	// FinishSignIn checks the browser's response against state, finding the account by the user handle the
	// passkey carries, and returns the account and the passkey as it is now; or ErrPasskeyRejected.
	FinishSignIn(site Site, state, response []byte, find func(handle []byte) (Account, error)) (Account, Passkey, error)
}

// ErrPasskeyRejected means a passkey response didn't check out: a wrong or reused challenge, another site, a bad
// signature, a passkey that isn't this account's, or one that looks copied.
var ErrPasskeyRejected = fault.Invalid("passkey", "couldn't be checked; please try again")

// ErrSignupClosed means the server takes no new accounts, because it has no sign-up code.
var ErrSignupClosed = fault.Invalid("signup", "is closed on this server")

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

// CheckSignupCode reports whether code is the server's sign-up code, in constant time. With no code set, sign-up is
// closed and nothing matches.
func CheckSignupCode(code string, want redact.Secret) error {
	if want.IsZero() {
		return ErrSignupClosed
	}
	got, expected := sha256.Sum256([]byte(code)), sha256.Sum256([]byte(want.Reveal()))
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

// SignUp is the second half of making an account: it checks the new passkey and saves the account with it.
func SignUp(ctx context.Context, repo Repository, cer Ceremonies, site Site, draft Account, state, response []byte,
	now time.Time,
) (Account, error) {
	key, err := cer.FinishRegistration(site, draft, state, response)
	if err != nil {
		return Account{}, err
	}
	return repo.CreateAccount(ctx, draft.Name, draft.Handle, key, now)
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
