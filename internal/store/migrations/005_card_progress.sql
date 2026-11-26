-- How far you've got with each card. Review is a simple loop: a right answer adds one to streak, a wrong one puts it
-- back to 0, and a card counts as learned while its streak is at least the study_streak setting (3 by default). The
-- totals are kept for showing progress. The row goes when its card goes.
CREATE TABLE card_progress (
    card_id     INTEGER PRIMARY KEY REFERENCES cards (id) ON DELETE CASCADE,
    streak      INTEGER NOT NULL CHECK (streak >= 0),
    right_count INTEGER NOT NULL CHECK (right_count >= 0),
    wrong_count INTEGER NOT NULL CHECK (wrong_count >= 0),
    reviewed_at INTEGER NOT NULL CHECK (reviewed_at > 0),
    -- A streak is a run of right answers, so it can never be longer than all of them.
    CHECK (streak <= right_count)
) STRICT;
