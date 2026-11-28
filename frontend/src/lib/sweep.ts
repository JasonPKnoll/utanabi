// The sweep across the sung line: how far through each of its pieces the singing is. Lyrics give a time for each line
// only, so each piece gets a share of the line's time by how long it takes to sing, its beats (morae): 羅針盤 is
// らしんばん, four beats, though it's three characters.

export interface Part {
  text: string;
  reading: string;
}

export interface Piece {
  ruby: Part[] | null;
}

export interface Timed {
  startMs: number | null;
}

// Small kana share a beat with the kana before them: きゃ is one beat.
const small = new Set("ぁぃぅぇぉゃゅょゎゕゖァィゥェォャュョヮヵヶ");

// A song's pace, per beat, when there aren't enough lines to measure it by, and the bounds of a measured one.
const defaultBeatMs = 400;
const fastestBeatMs = 100;
const slowestBeatMs = 600;
// The lines a pace is measured by: the middle of a song's lines, by time per beat from each line's start to the next's.
// Most lines run into the next, so the middle one shows how fast the song is sung, held notes and all; the few before a
// pause or an instrumental sit far above it. Short lines (a "yeah") say little, so they're left out.
const paceQuantile = 0.5;
const paceMinBeats = 3;
const paceMinLines = 4;
// How much slower than the song's pace a line may take before the rest of its time is taken to be a pause. Only a line
// well past it is cut short: cutting every line slower than the fastest raced the sweep ahead of the singing, since a
// singer holds notes and phrases lines unevenly.
const pauseFactor = 1.8;
// A singer breathes before the next line: a line's sweep ends this share of the gap before it starts, at most 0.4 s.
const breathShare = 0.12;
const longestBreathMs = 400;
const shortestLineMs = 1500;

// How many beats text takes to sing: one per kana (ー and っ included), two per kanji with no reading, one per group of
// vowels in Latin text, and one per digit. Punctuation and spaces take none.
export function beats(text: string): number {
  let n = 0;
  let inVowels = false;
  for (const ch of text) {
    const vowel = /[aeiouyAEIOUY]/.test(ch);
    if (vowel) {
      if (!inVowels) {
        n++;
      }
      inVowels = true;
      continue;
    }
    inVowels = false;
    if (/[ぁ-ゖァ-ヺー]/.test(ch)) {
      n += small.has(ch) ? 0 : 1;
    } else if (/[㐀-䶿一-鿿豈-﫿々]/.test(ch)) {
      n += 2;
    } else if (/[0-9０-９]/.test(ch)) {
      n++;
    }
  }
  return n;
}

// A piece's beats, from its readings where it has them.
export function pieceBeats(piece: Piece): number {
  return (piece.ruby ?? []).reduce((n, part) => n + beats(part.reading || part.text), 0);
}

// When line i is taken to end: a breath before the next timed line starts, unless that's well past what its beats take
// at the song's pace, when the rest is a pause. null for a line with no time.
export function lineEnd(
  lines: readonly Timed[],
  i: number,
  lineBeats: number,
  beatMs: number = defaultBeatMs,
): number | null {
  const start = lines[i]?.startMs;
  if (start === null || start === undefined) {
    return null;
  }
  const slowest = start + Math.max(shortestLineMs, lineBeats * beatMs * pauseFactor);
  const next = nextStart(lines, i);
  if (next === null) {
    return slowest;
  }
  const breath = Math.min(longestBreathMs, (next - start) * breathShare);
  return Math.min(next - breath, slowest);
}

// When the next timed line after line i starts, or null when none does.
function nextStart(lines: readonly Timed[], i: number): number | null {
  const start = lines[i]?.startMs;
  if (start === null || start === undefined) {
    return null;
  }
  return lines.slice(i + 1).find((l) => l.startMs !== null && l.startMs > start)?.startMs ?? null;
}

// How fast a song is sung, in ms per beat: the middle time per beat of its lines, from each line's start to the next's,
// between 100 and 600 ms. Usseewa is sung at about 210 ms a beat, Shiseikatsu at about 530.
// Without enough timed lines to tell, 400.
export function songPace(lines: readonly (Timed & { pieces: Piece[] | null })[]): number {
  const paces: number[] = [];
  lines.forEach((line, i) => {
    const next = nextStart(lines, i);
    const beats = (line.pieces ?? []).reduce((n, p) => n + pieceBeats(p), 0);
    if (line.startMs !== null && next !== null && beats >= paceMinBeats) {
      paces.push((next - line.startMs) / beats);
    }
  });
  if (paces.length < paceMinLines) {
    return defaultBeatMs;
  }
  paces.sort((a, b) => a - b);
  const pace = paces[Math.floor(paceQuantile * (paces.length - 1))] ?? defaultBeatMs;
  return Math.min(slowestBeatMs, Math.max(fastestBeatMs, pace));
}

// How far through each piece the singing is at ms, from 0 to 1, for a line sung from startMs to endMs. Pieces with no
// beats, such as punctuation, are done once the piece before them is.
export function sweep(pieces: readonly Piece[], startMs: number, endMs: number, ms: number): number[] {
  const weights = pieces.map(pieceBeats);
  const total = weights.reduce((a, b) => a + b, 0);
  if (total === 0 || endMs <= startMs) {
    return pieces.map(() => (ms >= startMs ? 1 : 0));
  }
  const sung = (Math.min(Math.max(ms, startMs), endMs) - startMs) / (endMs - startMs);
  let before = 0;
  return weights.map((w) => {
    const from = before / total;
    before += w;
    const to = before / total;
    if (w === 0) {
      return sung >= from ? 1 : 0;
    }
    return Math.min(1, Math.max(0, (sung - from) / (to - from)));
  });
}

// The sweep for line i of lines at lyric time ms, or null when there's none to show: no time to go by, or a line
// with no time.
export function lineSweep(
  lines: readonly (Timed & Piece & { pieces: Piece[] | null })[],
  i: number,
  ms: number | null,
  beatMs: number = defaultBeatMs,
): number[] | null {
  const line = lines[i];
  if (ms === null || !line || line.startMs === null) {
    return null;
  }
  const pieces = line.pieces ?? [];
  const end = lineEnd(
    lines,
    i,
    pieces.reduce((n, p) => n + pieceBeats(p), 0),
    beatMs,
  );
  return end === null ? null : sweep(pieces, line.startMs, end, ms);
}
