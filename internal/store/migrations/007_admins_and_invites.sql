-- Admins, and invites they make.
--
-- An admin can make and revoke invites. Being one is set only from the server's own machine, with the admin command
-- (go run ./internal/features/accounts/cmd/admin), never from the app, so nobody can make themselves one through it.
ALTER TABLE users ADD COLUMN admin INTEGER NOT NULL DEFAULT 0 CHECK (admin IN (0, 1));

-- An invite: a code that makes one account, once, before it runs out. The code is shown once, when it's made, and
-- only its SHA-256 is kept. used_by is who signed up with it; it stays set to when it was used if that account goes.
CREATE TABLE invites (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    code_hash  BLOB    NOT NULL UNIQUE CHECK (length(code_hash) = 32),
    created_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL CHECK (created_at > 0),
    expires_at INTEGER NOT NULL CHECK (expires_at > created_at),
    used_by    INTEGER REFERENCES users (id) ON DELETE SET NULL,
    used_at    INTEGER CHECK (used_at IS NULL OR used_at >= created_at),
    CHECK (used_by IS NULL OR used_at IS NOT NULL)
) STRICT;
