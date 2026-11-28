// Lyrics timed for the track that's playing: when a song's lyrics are timed for another version (its video, say), a
// lyrics source often has them timed for the track too. Hand-timed lyrics differ line by line, so no one offset
// lines up another version's; the track's own lyrics do.

export interface Candidate {
  synced: boolean;
  durationMs: number;
  songId: number;
}

// How near in length lyrics must be to the track to be timed for it: sources give lengths in whole seconds.
const sameLengthMs = 1500;
// How far off the song's lyrics must be before others are looked for.
const offMs = 1000;

// Whether lyrics lyricsMs long are timed for another version than a track trackMs long.
export function timedForAnother(lyricsMs: number, trackMs: number): boolean {
  return lyricsMs > 0 && trackMs > 0 && Math.abs(lyricsMs - trackMs) > offMs;
}

// The synced lyrics closest in length to the track, within a second and a half, other than the song's own; null when
// there are none.
export function forTrack<T extends Candidate>(found: readonly T[], trackMs: number, songId: number): T | null {
  let best: T | null = null;
  for (const c of found) {
    const off = Math.abs(c.durationMs - trackMs);
    if (!c.synced || c.songId === songId || off > sameLengthMs) {
      continue;
    }
    if (!best || off < Math.abs(best.durationMs - trackMs)) {
      best = c;
    }
  }
  return best;
}

// What to search a lyrics source for: the title, with the artist first when the title doesn't already name them
// ("Fujii Kaze - Shinunoga E-Wa").
export function searchFor(title: string, artist: string): string {
  return artist && !title.toLowerCase().includes(artist.toLowerCase()) ? `${artist} ${title}` : title;
}
