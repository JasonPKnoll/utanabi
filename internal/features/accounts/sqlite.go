package accounts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JasonPKnoll/utanabi/internal/identity"
	"github.com/JasonPKnoll/utanabi/internal/store"
)

// SQLite is the Repository backed by the app's database.
type SQLite struct {
	db *store.DB
}

// NewSQLite returns a Repository that keeps accounts in db.
func NewSQLite(db *store.DB) *SQLite {
	return &SQLite{db: db}
}

// CreateAccount saves a user and their first passkey together, and uses up their invite in the same transaction: an
// account never exists without a way in, and one invite never makes two accounts.
func (s *SQLite) CreateAccount(ctx context.Context, name string, handle []byte, key Passkey, invite []byte,
	now time.Time,
) (Account, error) {
	a := Account{Handle: handle, Name: name, Passkeys: []Passkey{key}}
	err := s.db.WithTx(ctx, func(q store.Querier) error {
		err := q.QueryRowContext(ctx, `INSERT INTO users (handle, name, created_at) VALUES (?, ?, ?) RETURNING id`,
			handle, name, now.UnixMilli()).Scan(&a.ID)
		if err != nil {
			return store.Translate(err)
		}
		if invite != nil {
			res, err := q.ExecContext(ctx, `
				UPDATE invites SET used_by = ?, used_at = ?
				WHERE code_hash = ? AND used_by IS NULL AND used_at IS NULL AND expires_at > ?`,
				a.ID, now.UnixMilli(), invite, now.UnixMilli())
			if err != nil {
				return store.Translate(err)
			}
			if n, err := res.RowsAffected(); err != nil || n != 1 {
				return ErrInviteGone
			}
		}
		return insertPasskey(ctx, q, a.ID, key, now)
	})
	if err != nil {
		return Account{}, err
	}
	return a, nil
}

// AccountByHandle returns the user with this user handle, with their passkeys.
func (s *SQLite) AccountByHandle(ctx context.Context, handle []byte) (Account, error) {
	return s.account(ctx, `SELECT id, handle, name, admin FROM users WHERE handle = ?`, handle)
}

// AccountByID returns the user with this ID, with their passkeys.
func (s *SQLite) AccountByID(ctx context.Context, id int64) (Account, error) {
	return s.account(ctx, `SELECT id, handle, name, admin FROM users WHERE id = ?`, id)
}

func (s *SQLite) account(ctx context.Context, query string, arg any) (Account, error) {
	q := s.db.Querier()
	var a Account
	if err := q.QueryRowContext(ctx, query, arg).Scan(&a.ID, &a.Handle, &a.Name, &a.Admin); err != nil {
		return Account{}, store.Translate(err)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id, public_key, attestation_type, transports, aaguid, sign_count, backup_eligible, backup_state
		FROM passkeys WHERE user_id = ? ORDER BY created_at`, a.ID)
	if err != nil {
		return Account{}, store.Translate(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k Passkey
		var transports string
		var signCount int64
		if err := rows.Scan(&k.ID, &k.PublicKey, &k.AttestationType, &transports, &k.AAGUID, &signCount,
			&k.BackupEligible, &k.BackupState); err != nil {
			return Account{}, store.Translate(err)
		}
		if err := json.Unmarshal([]byte(transports), &k.Transports); err != nil {
			return Account{}, fmt.Errorf("accounts: read passkey transports failed (%v)", err)
		}
		k.SignCount = uint32(signCount) //nolint:gosec // G115: stored from a uint32, checked non-negative.
		a.Passkeys = append(a.Passkeys, k)
	}
	return a, store.Translate(rows.Err())
}

// AddPasskey saves another passkey for a user.
func (s *SQLite) AddPasskey(ctx context.Context, userID int64, key Passkey, now time.Time) error {
	return insertPasskey(ctx, s.db.Querier(), userID, key, now)
}

func insertPasskey(ctx context.Context, q store.Querier, userID int64, key Passkey, now time.Time) error {
	transports := key.Transports
	if transports == nil {
		transports = []string{}
	}
	list, err := json.Marshal(transports)
	if err != nil {
		return fmt.Errorf("accounts: encode passkey transports failed (%v)", err)
	}
	var aaguid any
	if len(key.AAGUID) == 16 {
		aaguid = key.AAGUID
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO passkeys (id, user_id, public_key, attestation_type, transports, aaguid, sign_count,
			backup_eligible, backup_state, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		key.ID, userID, key.PublicKey, key.AttestationType, string(list), aaguid, int64(key.SignCount),
		key.BackupEligible, key.BackupState, now.UnixMilli())
	return store.Translate(err)
}

// UsedPasskey records a sign-in: the passkey's sign count and backup state now, and when it was used.
func (s *SQLite) UsedPasskey(ctx context.Context, keyID []byte, signCount uint32, backupState bool, now time.Time) error {
	res, err := s.db.Querier().ExecContext(ctx,
		`UPDATE passkeys SET sign_count = ?, backup_state = ?, last_used_at = ? WHERE id = ?`,
		int64(signCount), backupState, now.UnixMilli(), keyID)
	if err != nil {
		return store.Translate(err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return store.Translate(sql.ErrNoRows)
	}
	return nil
}

// CreateSession saves a session, and clears every session that has run out.
func (s *SQLite) CreateSession(ctx context.Context, tokenHash []byte, userID int64, now, expires time.Time) error {
	return s.db.WithTx(ctx, func(q store.Querier) error {
		if _, err := q.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.UnixMilli()); err != nil {
			return store.Translate(err)
		}
		_, err := q.ExecContext(ctx,
			`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			tokenHash, userID, now.UnixMilli(), expires.UnixMilli())
		return store.Translate(err)
	})
}

// SessionUser returns the user of a session that hasn't run out, and whether they're an admin.
func (s *SQLite) SessionUser(ctx context.Context, tokenHash []byte, now time.Time) (identity.User, error) {
	var u identity.User
	err := s.db.Querier().QueryRowContext(ctx, `
		SELECT u.id, u.name, u.admin FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now.UnixMilli()).Scan(&u.ID, &u.Name, &u.Admin)
	if err != nil {
		return identity.User{}, store.Translate(err)
	}
	return u, nil
}

// DeleteSession ends a session.
func (s *SQLite) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.Querier().ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return store.Translate(err)
}

// InviteUsable reports whether an invite is unused and hasn't run out.
func (s *SQLite) InviteUsable(ctx context.Context, codeHash []byte, now time.Time) (bool, error) {
	var ok bool
	err := s.db.Querier().QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM invites
			WHERE code_hash = ? AND used_by IS NULL AND used_at IS NULL AND expires_at > ?)`,
		codeHash, now.UnixMilli()).Scan(&ok)
	return ok, store.Translate(err)
}

// Invites returns every invite, newest first, with who made it and who used it.
func (s *SQLite) Invites(ctx context.Context) ([]Invite, error) {
	rows, err := s.db.Querier().QueryContext(ctx, `
		SELECT i.id, coalesce(c.name, ''), i.created_at, i.expires_at, coalesce(u.name, ''), coalesce(i.used_at, 0)
		FROM invites i
		LEFT JOIN users c ON c.id = i.created_by
		LEFT JOIN users u ON u.id = i.used_by
		ORDER BY i.id DESC`)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()
	list := []Invite{}
	for rows.Next() {
		var inv Invite
		var created, expires, used int64
		if err := rows.Scan(&inv.ID, &inv.CreatedBy, &created, &expires, &inv.UsedBy, &used); err != nil {
			return nil, store.Translate(err)
		}
		inv.CreatedAt, inv.ExpiresAt = time.UnixMilli(created), time.UnixMilli(expires)
		if used > 0 {
			inv.UsedAt = time.UnixMilli(used)
		}
		list = append(list, inv)
	}
	return list, store.Translate(rows.Err())
}

// CreateInvite saves an invite by its code's hash.
func (s *SQLite) CreateInvite(ctx context.Context, codeHash []byte, createdBy int64, now, expires time.Time) (
	Invite, error,
) {
	inv := Invite{CreatedAt: time.UnixMilli(now.UnixMilli()), ExpiresAt: time.UnixMilli(expires.UnixMilli())}
	err := s.db.Querier().QueryRowContext(ctx, `
		INSERT INTO invites (code_hash, created_by, created_at, expires_at) VALUES (?, ?, ?, ?)
		RETURNING id, (SELECT name FROM users WHERE id = ?)`,
		codeHash, createdBy, now.UnixMilli(), expires.UnixMilli(), createdBy).Scan(&inv.ID, &inv.CreatedBy)
	if err != nil {
		return Invite{}, store.Translate(err)
	}
	return inv, nil
}

// DeleteInvite removes an invite that hasn't been used. A used one stays, as the record of who joined with it.
func (s *SQLite) DeleteInvite(ctx context.Context, id int64) error {
	res, err := s.db.Querier().ExecContext(ctx, `DELETE FROM invites WHERE id = ? AND used_at IS NULL`, id)
	if err != nil {
		return store.Translate(err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return store.Translate(sql.ErrNoRows)
	}
	return nil
}

// SetAdmin makes a user an admin, or not. Only the admin command, run on the server's machine, calls it.
func (s *SQLite) SetAdmin(ctx context.Context, name string, admin bool) error {
	res, err := s.db.Querier().ExecContext(ctx, `UPDATE users SET admin = ? WHERE name = ?`, admin, name)
	if err != nil {
		return store.Translate(err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return store.Translate(sql.ErrNoRows)
	}
	return nil
}

// UserSummary is one account, for the admin command's list.
type UserSummary struct {
	Name      string
	Admin     bool
	CreatedAt time.Time
}

// Users returns every account, oldest first.
func (s *SQLite) Users(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.db.Querier().QueryContext(ctx, `SELECT name, admin, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, store.Translate(err)
	}
	defer rows.Close()
	var list []UserSummary
	for rows.Next() {
		var u UserSummary
		var created int64
		if err := rows.Scan(&u.Name, &u.Admin, &created); err != nil {
			return nil, store.Translate(err)
		}
		u.CreatedAt = time.UnixMilli(created)
		list = append(list, u)
	}
	return list, store.Translate(rows.Err())
}
