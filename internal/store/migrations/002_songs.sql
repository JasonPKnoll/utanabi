-- Songs and their lyrics, one row per sung line. The full design is in docs/data-model.md.
-- AUTOINCREMENT on tables that others point at: an ID is never reused, so an old link can't open a different row.

-- Where lyrics come from. A table instead of a CHECK list, so a new source is one INSERT in a later migration.
CREATE TABLE lyric_sources (
    code TEXT PRIMARY KEY CHECK (length(code) BETWEEN 1 AND 32 AND code NOT GLOB '*[^a-z0-9]*'),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100)
) STRICT, WITHOUT ROWID;

INSERT INTO lyric_sources (code, name) VALUES
    ('user', 'Your lyrics'),
    ('lrclib', 'LRCLIB'),
    ('netease', 'NetEase Cloud Music'),
    ('qqmusic', 'QQ Music');

CREATE TABLE songs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    title         TEXT    NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    title_reading TEXT    CHECK (title_reading IS NULL OR length(title_reading) BETWEEN 1 AND 400),
    artist        TEXT    NOT NULL DEFAULT '' CHECK (length(artist) <= 200),
    duration_ms   INTEGER CHECK (duration_ms IS NULL OR duration_ms > 0),
    source        TEXT    NOT NULL REFERENCES lyric_sources (code),
    -- The source's own ID for the song. Your own lyrics have none, and every other source must give one.
    source_ref    TEXT    CHECK (source_ref IS NULL OR length(source_ref) BETWEEN 1 AND 200),
    created_at    INTEGER NOT NULL CHECK (created_at > 0),
    updated_at    INTEGER NOT NULL CHECK (updated_at >= created_at),
    CHECK ((source = 'user') = (source_ref IS NULL)),
    UNIQUE (source, source_ref)
) STRICT;

-- One row per sung line, in order. Blank lines aren't stored: a verse break is a new stanza number.
CREATE TABLE lyric_lines (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    song_id  INTEGER NOT NULL REFERENCES songs (id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    stanza   INTEGER NOT NULL CHECK (stanza BETWEEN 0 AND position),
    text     TEXT    NOT NULL CHECK (
        length(text) BETWEEN 1 AND 500
        AND text = trim(text)
        AND instr(text, char(10)) = 0
        AND instr(text, char(13)) = 0
    ),
    -- When the line starts, in milliseconds from the start of the song. Only synced lyrics have it.
    start_ms INTEGER CHECK (start_ms IS NULL OR start_ms >= 0),
    UNIQUE (song_id, position)
) STRICT;
