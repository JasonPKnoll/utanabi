package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/common/redact"
	"github.com/JasonPKnoll/utanabi/internal/identity"
)

func TestName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
		ok       bool
	}{
		{"hana", "hana", true},
		{"  Kaze_Fujii-1 ", "kaze_fujii-1", true},
		{"ab", "", false},
		{strings.Repeat("a", 33), "", false},
		{"花子", "", false},
		{"no spaces", "", false},
		{"semi;colon", "", false},
	}
	for _, tt := range tests {
		got, err := Name(tt.in)
		if got != tt.want || (err == nil) != tt.ok {
			t.Errorf("Name(%q) = %q, %v, want %q (ok %v)", tt.in, got, err, tt.want, tt.ok)
		}
	}
}

func TestCheckSignupCode(t *testing.T) {
	t.Parallel()

	code := redact.New("a long sign-up code")
	if err := CheckSignupCode("a long sign-up code", code); err != nil {
		t.Errorf("right code: %v", err)
	}
	var invalid *fault.InvalidError
	if err := CheckSignupCode("a wrong code", code); !errors.As(err, &invalid) || invalid.Field != "code" {
		t.Errorf("wrong code: %v, want invalid code", err)
	}
	if err := CheckSignupCode("", redact.Secret{}); !errors.Is(err, ErrSignupClosed) {
		t.Errorf("no code set: %v, want sign-up closed", err)
	}
	if err := CheckSignupCode("anything", redact.Secret{}); !errors.Is(err, ErrSignupClosed) {
		t.Errorf("no code set, a code given: %v, want sign-up closed", err)
	}
}

func TestSessionTokens(t *testing.T) {
	t.Parallel()

	token, hash, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken: %v", err)
	}
	other, _, _ := NewSessionToken()
	if !validToken(token) || token == other || !bytes.Equal(hash, TokenHash(token)) || len(hash) != 32 {
		t.Errorf("token %q, hash %x: want a valid random token whose hash is TokenHash", token, hash)
	}
	if validToken(token[:42]) || validToken(token[:42]+"!") {
		t.Error("validToken accepted a short or odd token")
	}
	handle, err := NewHandle()
	if err != nil || len(handle) != 32 {
		t.Errorf("NewHandle = %x, %v, want 32 random bytes", handle, err)
	}
}

// fakeRepo keeps accounts in memory.
type fakeRepo struct {
	mu       sync.Mutex
	accounts []Account
	sessions map[string]session
	used     map[string]uint32
}

type session struct {
	user    identity.User
	expires time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{sessions: map[string]session{}, used: map[string]uint32{}}
}

func (f *fakeRepo) CreateAccount(_ context.Context, name string, handle []byte, key Passkey, _ time.Time) (Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.Name == name {
			return Account{}, fault.ErrConflict
		}
	}
	a := Account{ID: int64(len(f.accounts) + 1), Handle: handle, Name: name, Passkeys: []Passkey{key}}
	f.accounts = append(f.accounts, a)
	return a, nil
}

func (f *fakeRepo) AccountByHandle(_ context.Context, handle []byte) (Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if bytes.Equal(a.Handle, handle) {
			return a, nil
		}
	}
	return Account{}, fault.ErrNotFound
}

func (f *fakeRepo) AccountByID(_ context.Context, id int64) (Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return Account{}, fault.ErrNotFound
}

func (f *fakeRepo) AddPasskey(_ context.Context, userID int64, key Passkey, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, a := range f.accounts {
		if a.ID == userID {
			f.accounts[i].Passkeys = append(f.accounts[i].Passkeys, key)
			return nil
		}
	}
	return fault.ErrNotFound
}

func (f *fakeRepo) UsedPasskey(_ context.Context, keyID []byte, signCount uint32, _ bool, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.used[string(keyID)] = signCount
	return nil
}

func (f *fakeRepo) CreateSession(_ context.Context, hash []byte, userID int64, _, expires time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.ID == userID {
			f.sessions[string(hash)] = session{identity.User{ID: a.ID, Name: a.Name}, expires}
			return nil
		}
	}
	return fault.ErrNotFound
}

func (f *fakeRepo) SessionUser(_ context.Context, hash []byte, now time.Time) (identity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[string(hash)]
	if !ok || !now.Before(s.expires) {
		return identity.User{}, fault.ErrNotFound
	}
	return s.user, nil
}

func (f *fakeRepo) DeleteSession(_ context.Context, hash []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, string(hash))
	return nil
}

// fakeCeremonies stands in for WebAuthn: a response of "good" is a passkey that checks out, "good:<name>" signs in
// as that account, and anything else is rejected. The state holds the site it began on, so tests see the binding.
// It remembers each registered account's user handle, which a real passkey carries.
type fakeCeremonies struct {
	mu      sync.Mutex
	handles map[string][]byte
}

func newFakeCeremonies() *fakeCeremonies {
	return &fakeCeremonies{handles: map[string][]byte{}}
}

func (*fakeCeremonies) BeginRegistration(site Site, a Account) (json.RawMessage, []byte, error) {
	return json.RawMessage(`{"publicKey":{"rp":{"id":"` + site.ID + `"}}}`), []byte("reg:" + site.ID + ":" + a.Name), nil
}

func (f *fakeCeremonies) FinishRegistration(site Site, a Account, state, response []byte) (Passkey, error) {
	if string(state) != "reg:"+site.ID+":"+a.Name || string(response) != "good" {
		return Passkey{}, ErrPasskeyRejected
	}
	f.mu.Lock()
	f.handles[a.Name] = a.Handle
	f.mu.Unlock()
	return Passkey{ID: []byte("key-" + a.Name + "-" + strconv.Itoa(len(a.Passkeys))), PublicKey: []byte{1}}, nil
}

func (*fakeCeremonies) BeginSignIn(site Site, conditional bool) (json.RawMessage, []byte, error) {
	mediation := ""
	if conditional {
		mediation = `,"mediation":"conditional"`
	}
	return json.RawMessage(`{"publicKey":{"rpId":"` + site.ID + `"}` + mediation + `}`), []byte("signin:" + site.ID), nil
}

func (f *fakeCeremonies) FinishSignIn(site Site, state, response []byte, find func([]byte) (Account, error)) (
	Account, Passkey, error,
) {
	name, ok := strings.CutPrefix(string(response), "good:")
	if string(state) != "signin:"+site.ID || !ok {
		return Account{}, Passkey{}, ErrPasskeyRejected
	}
	f.mu.Lock()
	handle, known := f.handles[name]
	f.mu.Unlock()
	if !known {
		handle = []byte("no such handle")
	}
	a, err := find(handle)
	if err != nil {
		return Account{}, Passkey{}, err
	}
	return a, Passkey{ID: a.Passkeys[0].ID, SignCount: 7}, nil
}

func TestSignUpThenSignIn(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	cer := newFakeCeremonies()
	site := Site{ID: "localhost", Origin: "http://localhost:8080"}
	draft := Account{Handle: []byte("handle-hana"), Name: "hana"}
	_, state, _ := cer.BeginRegistration(site, draft)
	a, err := SignUp(t.Context(), repo, cer, site, draft, state, []byte("good"), time.Unix(1, 0))
	if err != nil || a.ID == 0 || len(a.Passkeys) != 1 {
		t.Fatalf("SignUp = %+v, %v, want an account with one passkey", a, err)
	}
	if _, err := SignUp(t.Context(), repo, cer, site, draft, state, []byte("bad"), time.Unix(1, 0)); !errors.Is(err, ErrPasskeyRejected) {
		t.Errorf("SignUp with a bad passkey: %v, want rejected", err)
	}

	_, state, _ = cer.BeginSignIn(site, false)
	got, err := SignIn(t.Context(), repo, cer, site, state, []byte("good:hana"), time.Unix(2, 0))
	if err != nil || got.ID != a.ID {
		t.Fatalf("SignIn = %+v, %v, want hana", got, err)
	}
	if repo.used[string(a.Passkeys[0].ID)] != 7 {
		t.Errorf("sign count recorded = %d, want 7", repo.used[string(a.Passkeys[0].ID)])
	}

	other := Site{ID: "example.ngrok-free.dev", Origin: "https://example.ngrok-free.dev"}
	if _, err := SignIn(t.Context(), repo, cer, other, state, []byte("good:hana"), time.Unix(2, 0)); !errors.Is(err, ErrPasskeyRejected) {
		t.Errorf("SignIn on another site with this site's challenge: %v, want rejected", err)
	}
}
