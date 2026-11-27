// Following a song along with Spotify: where the song is now, from the last answer about it, and which line that is.

export interface Timed {
  startMs: number | null;
}

// The line being sung at ms: the last one that has started. -1 before the first line, or for lyrics with no times.
export function lineAt(lines: readonly Timed[], ms: number): number {
  let at = -1;
  lines.forEach((line, i) => {
    if (line.startMs !== null && line.startMs <= ms) {
      at = i;
    }
  });
  return at;
}

// Whether the lyrics have times to follow along with: synced lyrics do.
export function hasTimes(lines: readonly Timed[]): boolean {
  return lines.some((line) => line.startMs !== null);
}

// Where the song is now: as far as Spotify last said, plus the time since, while it plays. Spotify is only asked
// every couple of seconds, so this counts on smoothly between its answers.
export function positionAt(progressMs: number, playing: boolean, askedAt: number, now: number): number {
  return playing ? progressMs + Math.max(0, now - askedAt) : progressMs;
}

// The track's web link, from its spotify: URI. A tap on it opens the Spotify app on a phone that has it, with no
// "Open in Spotify?" prompt from the browser, which a spotify: link always gets; elsewhere it opens Spotify's web
// player. "" when the URI isn't a track's.
export function trackLink(uri: string): string {
  const id = /^spotify:track:([A-Za-z0-9]+)$/.exec(uri)?.[1];
  return id ? `https://open.spotify.com/track/${id}` : "";
}

// Lyrics and their Spotify track can be timed apart: a line timed at t is sung at t + offset, so lyrics timed for a
// version with a longer intro have a negative offset.
export function trackTime(lyricsMs: number, offset: number): number {
  return Math.max(0, lyricsMs + offset);
}

export function lyricsTime(trackMs: number, offset: number): number {
  return trackMs - offset;
}

// The line after the one being sung at ms: the first that starts later. -1 when none does.
export function nextLine(lines: readonly Timed[], ms: number): number {
  return lines.findIndex((line) => line.startMs !== null && line.startMs > ms);
}

// The offset as you'd say it: "on time", or how much earlier or later than timed the lines come.
export function describeOffset(offset: number): string {
  const seconds = Math.abs(offset) / 1000;
  if (seconds < 0.05) {
    return "on time";
  }
  return `${seconds.toFixed(1)} s ${offset < 0 ? "earlier" : "later"}`;
}
