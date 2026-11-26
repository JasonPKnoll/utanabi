//go:build integration

package accounts

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/common/fault"
	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
	"github.com/JasonPKnoll/utanabi/internal/store/storetest"
)

func newRepo(t *testing.T) *SQLite {
	t.Helper()
	db := storetest.New(t)
	if err := db.Migrate(t.Context(), store.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewSQLite(db)
}

func key(id string) Passkey {
	return Passkey{
		ID: []byte(id + "-0123456789abcdef"), PublicKey: []byte{1, 2, 3}, AttestationType: "none",
		Transports: []string{"internal", "hybrid"}, AAGUID: bytes.Repeat([]byte{7}, 16), SignCount: 3,
		BackupEligible: true, BackupState: true,
	}
}

func TestAccountIsSavedWithItsPasskey(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	handle := bytes.Repeat([]byte{1}, 32)
	a, err := repo.CreateAccount(t.Context(), "hana", handle, key("phone"), time.UnixMilli(1))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	byHandle, err := repo.AccountByHandle(t.Context(), handle)
	if err != nil {
		t.Fatalf("AccountByHandle: %v", err)
	}
	byID, err := repo.AccountByID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("AccountByID: %v", err)
	}
	want := Account{ID: a.ID, Handle: handle, Name: "hana", Passkeys: []Passkey{key("phone")}}
	if !reflect.DeepEqual(byHandle, want) || !reflect.DeepEqual(byID, want) {
		t.Errorf("account\n by handle %+v\n by ID     %+v\n want      %+v", byHandle, byID, want)
	}

	if _, err := repo.CreateAccount(t.Context(), "hana", bytes.Repeat([]byte{2}, 32), key("other"), time.UnixMilli(1)); !errors.Is(err, fault.ErrConflict) {
		t.Errorf("a taken name: %v, want conflict", err)
	}
	if _, err := repo.AccountByHandle(t.Context(), bytes.Repeat([]byte{9}, 32)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("an unknown handle: %v, want not found", err)
	}
	if _, err := repo.AccountByID(t.Context(), 999); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("an unknown ID: %v, want not found", err)
	}
}

func TestPasskeysAreAddedAndRecorded(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	a, err := repo.CreateAccount(t.Context(), "hana", bytes.Repeat([]byte{1}, 32), key("phone"), time.UnixMilli(1))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	laptop := key("laptop")
	laptop.AAGUID, laptop.Transports = nil, nil
	if err := repo.AddPasskey(t.Context(), a.ID, laptop, time.UnixMilli(2)); err != nil {
		t.Fatalf("AddPasskey: %v", err)
	}
	if err := repo.AddPasskey(t.Context(), a.ID, laptop, time.UnixMilli(2)); !errors.Is(err, fault.ErrConflict) {
		t.Errorf("the same passkey twice: %v, want conflict", err)
	}
	if err := repo.UsedPasskey(t.Context(), laptop.ID, 9, false, time.UnixMilli(5)); err != nil {
		t.Fatalf("UsedPasskey: %v", err)
	}
	if err := repo.UsedPasskey(t.Context(), []byte("no such key-0123456"), 1, false, time.UnixMilli(5)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("recording an unknown passkey: %v, want not found", err)
	}
	got, err := repo.AccountByID(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("AccountByID: %v", err)
	}
	if len(got.Passkeys) != 2 || got.Passkeys[1].SignCount != 9 || got.Passkeys[1].BackupState ||
		got.Passkeys[1].AAGUID != nil || len(got.Passkeys[1].Transports) != 0 {
		t.Errorf("passkeys = %+v, want the phone, then the laptop at sign count 9", got.Passkeys)
	}
}

func TestSessions(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	a, err := repo.CreateAccount(t.Context(), "hana", bytes.Repeat([]byte{1}, 32), key("phone"), time.UnixMilli(1))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	now := time.UnixMilli(1_000_000)
	old, live := bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{4}, 32)
	if err := repo.CreateSession(t.Context(), old, a.ID, time.UnixMilli(10), time.UnixMilli(20)); err != nil {
		t.Fatalf("CreateSession old: %v", err)
	}
	if err := repo.CreateSession(t.Context(), live, a.ID, now, now.Add(SessionLife)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	u, err := repo.SessionUser(t.Context(), live, now.Add(time.Hour))
	if err != nil || u != (identity.User{ID: a.ID, Name: "hana"}) {
		t.Errorf("SessionUser = %+v, %v, want hana", u, err)
	}
	if _, err := repo.SessionUser(t.Context(), live, now.Add(SessionLife)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("a session at its end: %v, want not found", err)
	}
	// Making the live session cleared the one that had run out.
	if _, err := repo.SessionUser(t.Context(), old, time.UnixMilli(15)); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("the run-out session after cleanup: %v, want not found", err)
	}

	if err := repo.DeleteSession(t.Context(), live); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := repo.SessionUser(t.Context(), live, now); !errors.Is(err, fault.ErrNotFound) {
		t.Errorf("a deleted session: %v, want not found", err)
	}
	if err := repo.DeleteSession(t.Context(), live); err != nil {
		t.Errorf("deleting it again: %v, want no error", err)
	}
	if err := repo.CreateSession(t.Context(), live, 999, now, now.Add(time.Hour)); err == nil {
		t.Error("a session for no user was saved")
	}
}
