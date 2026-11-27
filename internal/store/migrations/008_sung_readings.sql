-- How a word is sung when that isn't how it's written: gikun, such as 羅針盤 written "compass" but sung コンパス, or
-- 地球 sung ほし. Its furigana and romaji follow the sung reading; its dictionary reading stays for meanings and cards.
-- Each owner's own: a song from a lyrics source is shared, so one person's reading never changes it for anyone else.
CREATE TABLE sung_readings (
    owner       INTEGER NOT NULL CHECK (owner >= 0),
    line_id     INTEGER NOT NULL REFERENCES lyric_lines (id) ON DELETE CASCADE,
    -- The word's tokens, and its text as written, so a reading is only used while the line's tokens still spell it.
    first_token INTEGER NOT NULL CHECK (first_token >= 0),
    last_token  INTEGER NOT NULL CHECK (last_token >= first_token),
    surface     TEXT    NOT NULL CHECK (length(surface) BETWEEN 1 AND 500),
    -- Hiragana or katakana, as it shows over the word, with ー and the iteration marks.
    reading     TEXT    NOT NULL CHECK (length(reading) BETWEEN 1 AND 100 AND reading NOT GLOB '*[^ぁ-ゖゝゞァ-ヺヽヾー]*'),
    PRIMARY KEY (owner, line_id, first_token)
) STRICT;

CREATE TRIGGER sung_readings_owner_insert BEFORE INSERT ON sung_readings
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: sung_readings.owner is not a user');
END;
CREATE TRIGGER sung_readings_owner_update BEFORE UPDATE OF owner ON sung_readings
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: sung_readings.owner is not a user');
END;

-- Removing a user removes their readings too.
CREATE TRIGGER users_delete_sung_readings AFTER DELETE ON users
BEGIN
    DELETE FROM sung_readings WHERE owner = OLD.id;
END;
