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

// A move you made on Spotify (a jump to a line, Play, Pause), held until Spotify's answers show it: for a second or
// two after a move, Spotify can still say where the song was, which would pull the lyrics back and forth.
export interface Move {
  trackMs: number;
  playing: boolean;
  // When it was made, on performance.now()'s clock.
  at: number;
}

// How far an answer can be from where a move put the song and still show it, and how long a move is held at most:
// after that, Spotify is trusted again, in case the move didn't take.
const caughtUpMs = 1500;
const holdMs = 5000;

// Whether an answer, got at now, shows the move: this song, playing or paused as asked, and about where the move put
// it by now.
export function caughtUp(
  move: Move,
  answer: { progressMs: number; playing: boolean; matches: boolean },
  now: number,
): boolean {
  if (now - move.at > holdMs) {
    return true;
  }
  if (!answer.matches || answer.playing !== move.playing) {
    return false;
  }
  return Math.abs(answer.progressMs - positionAt(move.trackMs, move.playing, move.at, now)) <= caughtUpMs;
}

// The song's clock, between Spotify's answers: where it was at `at` (performance.now()'s clock) and how fast it runs
// from there. It runs at 1 while it agrees with Spotify; when an answer says it's a little off, it runs a little
// faster or slower until it meets Spotify, so the lyrics never step back to sing part of a word again.
export interface Clock {
  at: number;
  ms: number;
  rate: number;
  playing: boolean;
}

// Further off than this, the clock jumps: the song was moved, not just counted a little wrong. steerMs is how long a
// small difference takes to make up.
const jumpMs = 1500;
const steerMs = 2000;

// Where the song is at now by clock.
export function clockAt(clock: Clock, now: number): number {
  return clock.playing ? clock.ms + Math.max(0, now - clock.at) * clock.rate : clock.ms;
}

// The clock set to the song at ms, now: exactly, or by steering towards it when it's only a little off.
export function steer(clock: Clock | null, ms: number, playing: boolean, now: number): Clock {
  if (!clock || !playing || !clock.playing) {
    return { at: now, ms, rate: 1, playing };
  }
  const shown = clockAt(clock, now);
  const off = ms - shown;
  if (Math.abs(off) > jumpMs) {
    return { at: now, ms, rate: 1, playing };
  }
  return { at: now, ms: shown, rate: Math.min(1.5, Math.max(0.5, 1 + off / steerMs)), playing };
}

// Where Spotify's answer puts the song now: its position was read somewhere between asking and hearing back, so it's
// taken to be as of halfway, and the song has played on since.
export function answeredAt(progressMs: number, playing: boolean, askedAt: number, heardAt: number): number {
  return playing ? progressMs + Math.max(0, heardAt - (askedAt + heardAt) / 2) : progressMs;
}
