-- Spotify, for following along: an owner's connection to their Spotify account, and the Spotify track each song is.
-- The app reads what's playing and asks Spotify to play, pause, and seek; it never touches the audio.

-- One per owner who has connected Spotify. The tokens are Spotify's, kept to call its API for the owner: the access
-- token runs out within an hour, and the refresh token gets a new one. They never leave the server.
CREATE TABLE spotify_links (
    owner         INTEGER PRIMARY KEY CHECK (owner >= 0),
    access_token  TEXT    NOT NULL CHECK (length(access_token) BETWEEN 1 AND 2000),
    refresh_token TEXT    NOT NULL CHECK (length(refresh_token) BETWEEN 1 AND 2000),
    expires_at    INTEGER NOT NULL CHECK (expires_at > 0),
    display_name  TEXT    NOT NULL DEFAULT '' CHECK (length(display_name) <= 200),
    connected_at  INTEGER NOT NULL CHECK (connected_at > 0)
) STRICT;

CREATE TRIGGER spotify_links_owner_insert BEFORE INSERT ON spotify_links
WHEN NEW.owner <> 0 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.owner)
BEGIN
    SELECT RAISE(ABORT, 'FOREIGN KEY constraint failed: spotify_links.owner is not a user');
END;

CREATE TRIGGER users_delete_spotify_links AFTER DELETE ON users
BEGIN
    DELETE FROM spotify_links WHERE owner = OLD.id;
END;

-- The Spotify track a song is, found once by its title, artist, and length, and shared, like the song.
CREATE TABLE spotify_tracks (
    song_id    INTEGER PRIMARY KEY REFERENCES songs (id) ON DELETE CASCADE,
    track_id   TEXT    NOT NULL CHECK (length(track_id) = 22 AND track_id NOT GLOB '*[^0-9A-Za-z]*'),
    matched_at INTEGER NOT NULL CHECK (matched_at > 0)
) STRICT;
