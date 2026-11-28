-- The permissions an owner's Spotify connection was given, as Spotify names them, space-separated: the page can play
-- a song itself (Spotify's Web Playback SDK) only with streaming, which connections made before 011 don't have, and
-- whose scopes weren't kept.
ALTER TABLE spotify_links ADD COLUMN scopes TEXT NOT NULL DEFAULT '' CHECK (length(scopes) <= 1000);
