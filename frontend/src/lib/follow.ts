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
