-- Each token's part of speech, a coarse code from internal/common/morph. Romaji needs it to read the particles は, へ,
-- and を as wa, e, and o, and to know which tokens join the word before them. Null when it isn't known.
ALTER TABLE line_tokens ADD COLUMN pos TEXT CHECK (pos IS NULL OR pos IN (
    'noun', 'pronoun', 'verb', 'adjective', 'adverb', 'particle', 'auxiliary', 'prefix', 'suffix', 'conjunction',
    'interjection', 'symbol', 'other'
));
