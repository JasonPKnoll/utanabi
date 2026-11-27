-- Secrets the server keeps for everyone, such as the YouTube API key an admin enters in Settings: each sealed
-- (AES-256-GCM, internal/common/secretbox) under the server's own key, which is kept in secret.key in the data folder
-- and never in this database, so a copy of the database alone doesn't give them away. Not anyone's own data, so not
-- scoped to an owner; only admins, and the admin command on the server's machine, change them.
CREATE TABLE server_secrets (
    name       TEXT    PRIMARY KEY CHECK (length(name) BETWEEN 1 AND 64 AND name NOT GLOB '*[^a-z0-9_]*'),
    -- The nonce, then the sealed value and its tag: at least 28 bytes.
    sealed     BLOB    NOT NULL CHECK (length(sealed) BETWEEN 28 AND 4096),
    updated_at INTEGER NOT NULL CHECK (updated_at > 0)
) STRICT;
