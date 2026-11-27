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

// The slowest a line is taken to be sung, per beat. A line ends when the next one starts, but not later than this, so
// a line before a long instrumental doesn't crawl through it.
const slowestBeatMs = 400;
const shortestLineMs = 1000;

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

// When line i is taken to end: when the next timed line starts, but no later than its beats at the slowest pace. null
// for a line with no time.
export function lineEnd(lines: readonly Timed[], i: number, lineBeats: number): number | null {
  const start = lines[i]?.startMs;
  if (start === null || start === undefined) {
    return null;
  }
  const slowest = start + Math.max(shortestLineMs, lineBeats * slowestBeatMs);
  const next = lines.slice(i + 1).find((l) => l.startMs !== null && l.startMs > start)?.startMs;
  return next === null || next === undefined ? slowest : Math.min(next, slowest);
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
  );
  return end === null ? null : sweep(pieces, line.startMs, end, ms);
}
