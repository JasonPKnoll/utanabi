-- Accounts for server mode, signed in with passkeys, and an owner for everything that is one person's.
--
-- An owner is a user's ID, or 0 for the device: the desktop and phone apps have no accounts, and everything they save
-- is the device's, as is whatever a server saved before it had accounts. SQLite can't add a foreign key with ALTER
-- TABLE, and 0 is no user, so triggers check owners instead.

-- A user. handle is the random user handle passkeys are made for: it never changes and is never shown. name is the
-- username, lowercase, so it's one account however it's typed.
CREATE TABLE users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT CHECK (id > 0),
    handle     BLOB    NOT NULL UNIQUE CHECK (length(handle) = 32),
    name       TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 3 AND 32 AND name NOT GLOB '*[^a-z0-9_-]*'),
    created_at INTEGER NOT NULL CHECK (created_at > 0)
) STRICT;

-- A passkey: the public half of a key pair whose private half stays on the user's device or in their password
-- manager. The server never sees a private key, and a public key can't sign anyone in. sign_count rises with each use
-- on authenticators that count, so a count that goes backwards shows a copied key. The backup flags say whether the
-- key may sync to another device (eligible) and whether it has (state).
CREATE TABLE passkeys (
    id               BLOB    PRIMARY KEY CHECK (length(id) BETWEEN 16 AND 1023),
    user_id          INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public_key       BLOB    NOT NULL CHECK (length(public_key) BETWEEN 1 AND 4096),
    attestation_type TEXT    NOT NULL CHECK (length(attestation_type) <= 64),
    transports       TEXT    NOT NULL CHECK (json_valid(transports) AND json_type(transports) = 'array'),
    aaguid           BLOB    CHECK (aaguid IS NULL OR length(aaguid) = 16),
    sign_count       INTEGER NOT NULL CHECK (sign_count >= 0),
    backup_eligible  INTEGER NOT NULL CHECK (backup_eligible IN (0, 1)),
    backup_state     INTEGER NOT NULL CHECK (backup_state IN (0, 1)),
    created_at       INTEGER NOT NULL CHECK (created_at > 0),
    last_used_at     INTEGER CHECK (last_used_at IS NULL OR last_used_at >= created_at)
) STRICT;
CREATE INDEX passkeys_by_user ON passkeys (user_id);

-- A signed-in session. The browser keeps a random token in a cookie; only its SHA-256 is stored, so the table alone
-- can't sign anyone in.
CREATE TABLE sessions (
    token_hash BLOB    PRIMARY KEY CHECK (length(token_hash) = 32),
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL CHECK (created_at > 0),
    expires_at INTEGER NOT NULL CHECK (expires_at > created_at)
) STRICT;
CREATE INDEX sessions_by_user ON sessions (user_id);

-- Each owner's library: the songs they've added. A song from a lyrics source is saved once, however many people add
-- it, so its analysis is shared; a song with your own lyrics is only ever in its owner's library.
CREATE TABLE library_songs (
    owner    INTEGER NOT NULL CHECK (owner >= 0),
    song_id  INTEGER NOT NULL REFERENCES songs (id) ON DELETE CASCADE,
    added_at INTEGER NOT NULL CHECK (added_at > 0),
    PRIMARY KEY (owner, song_id)
) STRICT;
CREATE INDEX library_songs_by_song ON library_songs (song_id);

-- Every song saved so far is the device's.
INSERT INTO library_songs (owner, song_id, added_at) SELECT 0, id, created_at FROM songs;

-- A song leaves the database when the last library holding it lets it go.
CREATE TRIGGER library_songs_last_one AFTER DELETE ON library_songs
WHEN NOT EXISTS (SELECT 1 FROM library_songs WHERE song_id = OLD.song_id)
BEGIN
    DELETE FROM songs WHERE id = OLD.song_id;
END;

-- Cards are one owner's, with one card per meaning, grammar point, and line for each owner.
ALTER TABLE cards ADD COLUMN owner INTEGER NOT NULL DEFAULT 0 CHECK (owner >= 0);
DROP INDEX cards_one_per_sense;
DROP INDEX cards_one_per_grammar_point;
DROP INDEX cards_one_per_line;
CREATE UNIQUE INDEX cards_one_per_sense ON cards (owner, entry_id, sense_index) WHERE kind = 'word';
CREATE UNIQUE INDEX cards_one_per_grammar_point ON cards (owner, grammar_point_id) WHERE kind = 'grammar';
CREATE UNIQUE INDEX cards_one_per_line ON cards (owner, line_id) WHERE kind = 'line';

-- An owner other than the device must be a user.
CREATE TRIGGER library_songs_owner_insert BEFORE INSERT ON library_songs
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: library_songs.owner is not a user');
END;
CREATE TRIGGER library_songs_owner_update BEFORE UPDATE OF owner ON library_songs
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: library_songs.owner is not a user');
END;
CREATE TRIGGER cards_owner_insert BEFORE INSERT ON cards
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: cards.owner is not a user');
END;
CREATE TRIGGER cards_owner_update BEFORE UPDATE OF owner ON cards
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: cards.owner is not a user');
END;

-- Removing a user removes everything that was theirs: passkeys and sessions by cascade, and their library and cards
-- here.
CREATE TRIGGER users_delete_owned AFTER DELETE ON users
BEGIN
    DELETE FROM library_songs WHERE owner = OLD.id;
    DELETE FROM cards WHERE owner = OLD.id;
END;
