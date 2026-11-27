-- YouTube, for playing a song's video in the page so its lyrics follow it: each owner's video for their songs, found
-- by searching YouTube or pasted as a link. Each owner's own, since the song is shared.
CREATE TABLE youtube_videos (
    owner       INTEGER NOT NULL CHECK (owner >= 0),
    song_id     INTEGER NOT NULL REFERENCES songs (id) ON DELETE CASCADE,
    -- YouTube's 11-character video ID.
    video_id    TEXT    NOT NULL CHECK (length(video_id) = 11 AND video_id NOT GLOB '*[^0-9A-Za-z_-]*'),
    title       TEXT    NOT NULL DEFAULT '' CHECK (length(title) <= 300),
    channel     TEXT    NOT NULL DEFAULT '' CHECK (length(channel) <= 200),
    duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms > 0),
    -- How far the owner has shifted the lyrics to line up with the video: a line timed at t is sung at t + offset_ms.
    offset_ms   INTEGER NOT NULL DEFAULT 0 CHECK (offset_ms BETWEEN -60000 AND 60000),
    -- 1 for a video the owner pasted, 0 for one search found.
    chosen      INTEGER NOT NULL DEFAULT 0 CHECK (chosen IN (0, 1)),
    matched_at  INTEGER NOT NULL CHECK (matched_at > 0),
    PRIMARY KEY (owner, song_id)
) STRICT;

CREATE TRIGGER youtube_videos_owner_insert BEFORE INSERT ON youtube_videos
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: youtube_videos.owner is not a user');
END;
CREATE TRIGGER youtube_videos_owner_update BEFORE UPDATE OF owner ON youtube_videos
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: youtube_videos.owner is not a user');
END;

-- Removing a user removes their videos too.
CREATE TRIGGER users_delete_youtube_videos AFTER DELETE ON users
BEGIN
    DELETE FROM youtube_videos WHERE owner = OLD.id;
END;
