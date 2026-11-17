-- Songs the user is learning, with their lyrics. source says where the lyrics came from: 'user' for lyrics the user
-- typed, and later the name of an online lyrics source. AUTOINCREMENT never reuses a deleted song's ID, so an old link
-- to a song can't open a different one.
CREATE TABLE songs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    artist     TEXT    NOT NULL DEFAULT '',
    lyrics     TEXT    NOT NULL,
    source     TEXT    NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
