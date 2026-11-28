-- Lining lyrics up with the Spotify track: lyrics are often timed for another cut of the song, such as its video,
-- whose intro is a few seconds longer or shorter.

-- The track's length, so the lyrics can be shifted to fit it before it plays. Tracks matched before 010 learn theirs
-- the first time they play.
ALTER TABLE spotify_tracks ADD COLUMN duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms > 0);

-- How far an owner has shifted a song's lyrics to line them up with its track: a line timed at t is sung at
-- t + offset_ms. Each owner's own, like sung readings, since the song is shared.
CREATE TABLE lyric_offsets (
    owner     INTEGER NOT NULL CHECK (owner >= 0),
    song_id   INTEGER NOT NULL REFERENCES songs (id) ON DELETE CASCADE,
    offset_ms INTEGER NOT NULL CHECK (offset_ms BETWEEN -60000 AND 60000),
    PRIMARY KEY (owner, song_id)
) STRICT;

CREATE TRIGGER lyric_offsets_owner_insert BEFORE INSERT ON lyric_offsets
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: lyric_offsets.owner is not a user');
END;
CREATE TRIGGER lyric_offsets_owner_update BEFORE UPDATE OF owner ON lyric_offsets
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: lyric_offsets.owner is not a user');
END;

-- Removing a user removes their offsets too.
CREATE TRIGGER users_delete_lyric_offsets AFTER DELETE ON users
BEGIN
    DELETE FROM lyric_offsets WHERE owner = OLD.id;
END;
