-- What the app learns from each line: translations, tokens for furigana, words with their meaning and grammar,
-- conjugation steps, and flashcards. The full design is in docs/data-model.md.
--
-- Some IDs point into dictionary.db, a separate read-only file: entry_id and sense_index (JMdict) and
-- grammar_point_id (our grammar list). They aren't foreign keys, because SQLite can't enforce those across files;
-- the CHECKs here only make sure they are well formed.

-- A translation of one line, written as a sentence. style says how close it stays to the Japanese: 'literal' follows
-- its words and order ("Night's sky in, the stars shine"), 'natural' reads as fluent English ("Stars shine in the
-- night sky"). origin says who wrote it: 'user' (you), 'ai' (a language model, named in model), or 'source' (a lyrics
-- source, named in source). A word-by-word gloss is never stored here: it is computed from line_words and the
-- dictionary, so it always matches the analysis.
CREATE TABLE line_translations (
    id         INTEGER PRIMARY KEY,
    line_id    INTEGER NOT NULL REFERENCES lyric_lines (id) ON DELETE CASCADE,
    lang       TEXT    NOT NULL CHECK (length(lang) BETWEEN 2 AND 8 AND lang NOT GLOB '*[^a-z-]*'),
    style      TEXT    NOT NULL CHECK (style IN ('literal', 'natural')),
    origin     TEXT    NOT NULL CHECK (origin IN ('user', 'ai', 'source')),
    model      TEXT    CHECK (model IS NULL OR length(model) BETWEEN 1 AND 100),
    source     TEXT    REFERENCES lyric_sources (code),
    text       TEXT    NOT NULL CHECK (length(text) BETWEEN 1 AND 1000),
    created_at INTEGER NOT NULL CHECK (created_at > 0),
    -- An AI translation always names its model, a source's always names the source, and yours names neither, so one
    -- kind can never pass for another.
    CHECK ((origin = 'ai') = (model IS NOT NULL)),
    CHECK ((origin = 'source') = (source IS NOT NULL)),
    -- One of each per line and language. When several exist the app shows yours first, then a source's, then AI.
    UNIQUE (line_id, lang, style, origin)
) STRICT;

-- The tokenizer's pieces of a line, kept only for furigana. The surfaces of a line, in order, join back into exactly
-- its text; the app checks that before saving.
CREATE TABLE line_tokens (
    id             INTEGER PRIMARY KEY,
    line_id        INTEGER NOT NULL REFERENCES lyric_lines (id) ON DELETE CASCADE,
    position       INTEGER NOT NULL CHECK (position >= 0),
    surface        TEXT    NOT NULL CHECK (length(surface) BETWEEN 1 AND 500),
    -- Hiragana only, with ー and the iteration marks ゝ ゞ. Null for punctuation and Latin text.
    reading        TEXT    CHECK (reading IS NULL OR (length(reading) BETWEEN 1 AND 500 AND reading NOT GLOB '*[^ぁ-ゖゝゞー]*')),
    reading_source TEXT    NOT NULL DEFAULT 'auto' CHECK (reading_source IN ('auto', 'user')),
    UNIQUE (line_id, position)
) STRICT;

-- The learnable units of a line. Each covers a run of tokens: 探している covers 探し, て, いる.
CREATE TABLE line_words (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    line_id          INTEGER NOT NULL REFERENCES lyric_lines (id) ON DELETE CASCADE,
    first_token      INTEGER NOT NULL CHECK (first_token >= 0),
    last_token       INTEGER NOT NULL CHECK (last_token >= first_token),
    kind             TEXT    NOT NULL CHECK (kind IN ('content', 'particle', 'expression', 'auxiliary')),
    -- The dictionary form (探す for 探している) and its hiragana. Words in Latin letters have no reading.
    lemma            TEXT    NOT NULL CHECK (length(lemma) BETWEEN 1 AND 100),
    lemma_reading    TEXT    CHECK (lemma_reading IS NULL OR (length(lemma_reading) BETWEEN 1 AND 200 AND lemma_reading NOT GLOB '*[^ぁ-ゖゝゞー]*')),
    -- How the word conjugates: a JMdict code such as v5s, v1, vs-i, vk, adj-i, adj-na. Go checks it against the
    -- full list; a pattern here means a new code doesn't need a table rebuild.
    conj_class       TEXT    CHECK (conj_class IS NULL OR (length(conj_class) BETWEEN 2 AND 10 AND conj_class GLOB '[av]*' AND conj_class NOT GLOB '*[^a-z0-9-]*')),
    -- The JMdict entry, and which of its meanings applies in this line.
    entry_id         INTEGER CHECK (entry_id IS NULL OR entry_id > 0),
    sense_index      INTEGER CHECK (sense_index IS NULL OR sense_index >= 0),
    -- What a particle or expression does here, such as ni-place for に in 空に.
    grammar_point_id TEXT    CHECK (grammar_point_id IS NULL OR (length(grammar_point_id) BETWEEN 1 AND 64 AND grammar_point_id NOT GLOB '*[^a-z0-9-]*')),
    analysis_source  TEXT    NOT NULL DEFAULT 'auto' CHECK (analysis_source IN ('auto', 'user')),
    CHECK (sense_index IS NULL OR entry_id IS NOT NULL),
    CHECK (kind <> 'particle' OR grammar_point_id IS NOT NULL),
    UNIQUE (line_id, first_token)
) STRICT;

CREATE INDEX line_words_by_sense ON line_words (entry_id, sense_index) WHERE entry_id IS NOT NULL;
CREATE INDEX line_words_by_grammar ON line_words (grammar_point_id) WHERE grammar_point_id IS NOT NULL;

-- A word must cover tokens that exist, and the words of a line must not overlap.
CREATE TRIGGER line_words_check_insert BEFORE INSERT ON line_words
BEGIN
    SELECT RAISE(ABORT, 'line word ends past the last token')
    WHERE NOT EXISTS (SELECT 1 FROM line_tokens WHERE line_id = NEW.line_id AND position = NEW.last_token);
    SELECT RAISE(ABORT, 'line words overlap')
    WHERE EXISTS (
        SELECT 1 FROM line_words
        WHERE line_id = NEW.line_id AND first_token <= NEW.last_token AND last_token >= NEW.first_token
    );
END;

CREATE TRIGGER line_words_check_update BEFORE UPDATE OF line_id, first_token, last_token ON line_words
BEGIN
    SELECT RAISE(ABORT, 'line word ends past the last token')
    WHERE NOT EXISTS (SELECT 1 FROM line_tokens WHERE line_id = NEW.line_id AND position = NEW.last_token);
    SELECT RAISE(ABORT, 'line words overlap')
    WHERE EXISTS (
        SELECT 1 FROM line_words
        WHERE line_id = NEW.line_id AND id <> OLD.id
          AND first_token <= NEW.last_token AND last_token >= NEW.first_token
    );
END;

-- How a word got from its dictionary form to the form in the line, one conjugation per step, in order:
-- 食べさせられなかった is causative, passive, negative, past.
CREATE TABLE word_inflections (
    word_id          INTEGER NOT NULL REFERENCES line_words (id) ON DELETE CASCADE,
    step             INTEGER NOT NULL CHECK (step >= 0),
    grammar_point_id TEXT    NOT NULL CHECK (length(grammar_point_id) BETWEEN 1 AND 64 AND grammar_point_id NOT GLOB '*[^a-z0-9-]*'),
    PRIMARY KEY (word_id, step)
) STRICT, WITHOUT ROWID;

-- Steps count up from 0 with no gaps.
CREATE TRIGGER word_inflections_check_step BEFORE INSERT ON word_inflections
WHEN NEW.step > 0
BEGIN
    SELECT RAISE(ABORT, 'inflection steps must count up from 0 with no gaps')
    WHERE NOT EXISTS (SELECT 1 FROM word_inflections WHERE word_id = NEW.word_id AND step = NEW.step - 1);
END;

-- Flashcards: one meaning of a word, one grammar point, or one line.
CREATE TABLE cards (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    kind             TEXT    NOT NULL CHECK (kind IN ('word', 'grammar', 'line')),
    entry_id         INTEGER CHECK (entry_id IS NULL OR entry_id > 0),
    sense_index      INTEGER CHECK (sense_index IS NULL OR sense_index >= 0),
    grammar_point_id TEXT    CHECK (grammar_point_id IS NULL OR (length(grammar_point_id) BETWEEN 1 AND 64 AND grammar_point_id NOT GLOB '*[^a-z0-9-]*')),
    -- A line card's line. The card goes when the line goes.
    line_id          INTEGER REFERENCES lyric_lines (id) ON DELETE CASCADE,
    -- The line a word or grammar card was saved from, for context. The card stays when the line goes.
    context_line_id  INTEGER REFERENCES lyric_lines (id) ON DELETE SET NULL,
    -- A copy of the word or pattern as saved, so the card still reads right after a dictionary update.
    label            TEXT    NOT NULL CHECK (length(label) BETWEEN 1 AND 500),
    label_reading    TEXT    CHECK (label_reading IS NULL OR length(label_reading) BETWEEN 1 AND 500),
    created_at       INTEGER NOT NULL CHECK (created_at > 0),
    CHECK (CASE kind
        WHEN 'word' THEN entry_id IS NOT NULL AND sense_index IS NOT NULL
            AND grammar_point_id IS NULL AND line_id IS NULL
        WHEN 'grammar' THEN grammar_point_id IS NOT NULL
            AND entry_id IS NULL AND sense_index IS NULL AND line_id IS NULL
        WHEN 'line' THEN line_id IS NOT NULL AND context_line_id IS NULL
            AND entry_id IS NULL AND sense_index IS NULL AND grammar_point_id IS NULL
    END)
) STRICT;

-- One card per meaning, per grammar point, and per line.
CREATE UNIQUE INDEX cards_one_per_sense ON cards (entry_id, sense_index) WHERE kind = 'word';
CREATE UNIQUE INDEX cards_one_per_grammar_point ON cards (grammar_point_id) WHERE kind = 'grammar';
CREATE UNIQUE INDEX cards_one_per_line ON cards (line_id) WHERE kind = 'line';
CREATE INDEX cards_by_context_line ON cards (context_line_id) WHERE context_line_id IS NOT NULL;
