-- When the words of a line are sung: the time each starts, by the character it starts at. Lyrics sources give a time
-- per line only, so without these the sweep across a line guesses at its words. They come from a hand-timed lyrics
-- database (source 'amll') or from tapping along with the song (source 'user'), and are in the lyrics' own time,
-- before the owner's offset onto the track.
-- Each owner's own, as the offset and sung readings are: a song from a lyrics source is shared, and one person's
-- timing never changes it for anyone else.
CREATE TABLE word_times (
    owner    INTEGER NOT NULL CHECK (owner >= 0),
    line_id  INTEGER NOT NULL REFERENCES lyric_lines (id) ON DELETE CASCADE,
    -- The character of the line's text the word starts at, counted from 0 in Unicode code points.
    at       INTEGER NOT NULL CHECK (at BETWEEN 0 AND 499),
    start_ms INTEGER NOT NULL CHECK (start_ms BETWEEN 0 AND 3600000),
    source   TEXT    NOT NULL CHECK (source IN ('amll', 'user')),
    PRIMARY KEY (owner, line_id, at)
) STRICT, WITHOUT ROWID;

CREATE TRIGGER word_times_owner_insert BEFORE INSERT ON word_times
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: word_times.owner is not a user');
END;
CREATE TRIGGER word_times_owner_update BEFORE UPDATE OF owner ON word_times
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: word_times.owner is not a user');
END;

-- Removing a user removes their timings too.
CREATE TRIGGER users_delete_word_times AFTER DELETE ON users
BEGIN
    DELETE FROM word_times WHERE owner = OLD.id;
END;
