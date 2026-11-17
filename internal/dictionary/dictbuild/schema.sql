-- dictionary.db: the read-only reference database, built once by dictbuild and replaced whole on update. A change here
-- bumps dictionary.SchemaVersion, so the app refuses a file built for another layout.

CREATE TABLE meta (
    key   TEXT NOT NULL PRIMARY KEY
          CHECK (key IN ('schema_version', 'jmdict_date', 'jmdict_simplified_version', 'built_at')),
    value TEXT NOT NULL CHECK (length(value) BETWEEN 1 AND 100)
) STRICT;

-- JMdict's codes for parts of speech, usage notes, fields, and dialects, with their descriptions, so the app can show
-- a code such as v5s in words.
CREATE TABLE tags (
    code        TEXT NOT NULL PRIMARY KEY CHECK (length(code) BETWEEN 1 AND 30),
    description TEXT NOT NULL CHECK (length(description) BETWEEN 1 AND 500)
) STRICT;

-- One row per JMdict entry. id is JMdict's own entry number (ent_seq), which utanabi.db stores as entry_id.
CREATE TABLE entries (
    id INTEGER PRIMARY KEY CHECK (id > 0)
) STRICT;

-- Every spelling of an entry paired with a reading: one row per reading of each kanji spelling, then one row per kana
-- spelling. reading is hiragana, so a lookup by a tokenizer's reading matches katakana words too.
CREATE TABLE entry_forms (
    entry_id INTEGER NOT NULL REFERENCES entries (id),
    position INTEGER NOT NULL CHECK (position >= 0),
    form     TEXT    NOT NULL CHECK (length(form) BETWEEN 1 AND 100),
    reading  TEXT    NOT NULL CHECK (length(reading) BETWEEN 1 AND 100),
    is_kanji INTEGER NOT NULL CHECK (is_kanji IN (0, 1)),
    common   INTEGER NOT NULL CHECK (common IN (0, 1)),
    tags     TEXT    NOT NULL CHECK (json_valid(tags) AND json_type(tags) = 'array'),
    PRIMARY KEY (entry_id, position),
    UNIQUE (entry_id, form, reading)
) STRICT, WITHOUT ROWID;

CREATE INDEX entry_forms_by_form ON entry_forms (form, reading);
CREATE INDEX entry_forms_by_reading ON entry_forms (reading);

-- Each meaning of an entry, in JMdict order from 0. utanabi.db stores sense_index to say which meaning a word has in
-- a line. Every list column is a JSON array of strings; the codes in pos double as conjugation classes.
CREATE TABLE senses (
    entry_id         INTEGER NOT NULL REFERENCES entries (id),
    sense_index      INTEGER NOT NULL CHECK (sense_index >= 0),
    glosses          TEXT    NOT NULL CHECK (json_valid(glosses) AND json_type(glosses) = 'array'
                                             AND json_array_length(glosses) >= 1),
    pos              TEXT    NOT NULL CHECK (json_valid(pos) AND json_type(pos) = 'array'),
    misc             TEXT    NOT NULL CHECK (json_valid(misc) AND json_type(misc) = 'array'),
    notes            TEXT    NOT NULL CHECK (json_valid(notes) AND json_type(notes) = 'array'),
    fields           TEXT    NOT NULL CHECK (json_valid(fields) AND json_type(fields) = 'array'),
    dialects         TEXT    NOT NULL CHECK (json_valid(dialects) AND json_type(dialects) = 'array'),
    applies_to_kanji TEXT    NOT NULL CHECK (json_valid(applies_to_kanji) AND json_type(applies_to_kanji) = 'array'),
    applies_to_kana  TEXT    NOT NULL CHECK (json_valid(applies_to_kana) AND json_type(applies_to_kana) = 'array'),
    PRIMARY KEY (entry_id, sense_index)
) STRICT, WITHOUT ROWID;

-- Our own grammar list, from internal/dictionary/grammar.json. One row per use of a particle, per conjugation, and per
-- pattern. The id shape matches the grammar_point_id checks in utanabi.db.
CREATE TABLE grammar_points (
    id          TEXT NOT NULL PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 64 AND id NOT GLOB '*[^a-z0-9-]*'),
    kind        TEXT NOT NULL CHECK (kind IN ('particle', 'conjugation', 'pattern')),
    pattern     TEXT NOT NULL CHECK (length(pattern) BETWEEN 1 AND 50),
    title       TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
    meaning     TEXT NOT NULL CHECK (length(meaning) BETWEEN 1 AND 200),
    explanation TEXT NOT NULL CHECK (length(explanation) BETWEEN 1 AND 2000),
    formation   TEXT CHECK (formation IS NULL OR length(formation) BETWEEN 1 AND 2000),
    standard_id TEXT REFERENCES grammar_points (id) CHECK (standard_id IS NULL OR standard_id <> id),
    CHECK (kind <> 'conjugation' OR formation IS NOT NULL)
) STRICT;
