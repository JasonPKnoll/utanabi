// The sweep across the sung line: how far through each of its pieces the singing is. A line with word times, found
// or tapped, sweeps each word from when it starts to when the next does. Otherwise lyrics give a time for each line
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

// When a word of a line starts: at is the character of the line's text it starts at, in code points.
export interface WordTime {
  at: number;
  ms: number;
}

// A line as the sweep reads it: its start, its pieces, and, when it has them, its text and word times.
export interface SweptLine extends Timed, Piece {
  pieces: Piece[] | null;
  text?: string;
  times?: WordTime[] | null;
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
// How much slower than the song's pace a line may take before the rest of its time is taken to be a pause. These
// settings, and the breath and the held last note below, are tuned on real word timings: sweep.real.test.ts scores
// them against 56 Japanese songs timed word by word.
const pauseFactor = 1.3;
// A singer breathes before the next line: a line's sweep ends this share of the gap before it starts, at most 0.7 s.
const breathShare = 0.12;
const longestBreathMs = 700;
// A singer holds a line's last note: it takes a beat more than its kana.
const finalHold = 1;
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

// How far through each piece the singing is at ms, from 0 to 1, for a line sung from startMs to endMs, its last note
// held a beat longer. Pieces with no beats, such as punctuation, are done once the piece before them is.
export function sweep(pieces: readonly Piece[], startMs: number, endMs: number, ms: number): number[] {
  const weights = pieces.map(pieceBeats);
  const last = weights.findLastIndex((w) => w > 0);
  if (last >= 0) {
    weights[last] = (weights[last] ?? 0) + finalHold;
  }
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

// Where each piece starts in text, in code points: each piece's text is looked for after the one before it, since
// pieces cover the line in order. A piece that isn't found is taken to start where the one before ended.
export function pieceOffsets(text: string, pieces: readonly Piece[]): number[] {
  const chars = Array.from(text);
  let cursor = 0;
  return pieces.map((piece) => {
    const own = Array.from((piece.ruby ?? []).map((part) => part.text).join(""));
    let at = cursor;
    for (let i = cursor; own.length > 0 && i + own.length <= chars.length; i++) {
      if (own.every((c, k) => chars[i + k] === c)) {
        at = i;
        break;
      }
    }
    cursor = at + own.length;
    return at;
  });
}

// The beats of each piece, the last sung one held a beat longer, as the sweep spreads a line's time.
function weightsOf(pieces: readonly Piece[]): number[] {
  const weights = pieces.map(pieceBeats);
  const last = weights.findLastIndex((w) => w > 0);
  if (last >= 0) {
    weights[last] = (weights[last] ?? 0) + finalHold;
  }
  return weights;
}

// When each piece starts, for a line sung from startMs to endMs with some word times known: a piece with a time
// starts then, and the rest are spread between the known ones by their beats.
export function pieceStarts(
  pieces: readonly Piece[],
  offsets: readonly number[],
  times: readonly WordTime[],
  startMs: number,
  endMs: number,
): number[] {
  const weights = weightsOf(pieces);
  const before: number[] = [];
  let total = 0;
  for (const w of weights) {
    before.push(total);
    total += w;
  }
  const lengths = pieces.map((p) => Array.from((p.ruby ?? []).map((part) => part.text).join("")).length);
  // Anchors, by beat: the known starts, in order and never going back, between the line's start and end.
  const anchors: { beat: number; ms: number }[] = [];
  pieces.forEach((_, j) => {
    const from = offsets[j] ?? 0;
    const known =
      (weights[j] ?? 0) > 0 ? times.find((t) => t.at >= from && t.at < from + Math.max(1, lengths[j] ?? 0)) : undefined;
    if (known) {
      const floor = anchors.at(-1)?.ms ?? startMs;
      anchors.push({ beat: before[j] ?? 0, ms: Math.min(Math.max(known.ms, floor), endMs) });
    }
  });
  if (anchors[0]?.beat !== 0) {
    anchors.unshift({ beat: 0, ms: Math.min(startMs, anchors[0]?.ms ?? startMs) });
  }
  if ((anchors.at(-1)?.beat ?? 0) < total) {
    anchors.push({ beat: total, ms: Math.max(endMs, anchors.at(-1)?.ms ?? endMs) });
  }
  return before.map((beat) => {
    const k = anchors.findLastIndex((a) => a.beat <= beat);
    const from = anchors[k] ?? { beat: 0, ms: startMs };
    const to = anchors[k + 1];
    if (!to || to.beat === from.beat) {
      return from.ms;
    }
    return from.ms + ((to.ms - from.ms) * (beat - from.beat)) / (to.beat - from.beat);
  });
}

// How far through each piece the singing is at ms, from 0 to 1, for pieces starting at starts and the line ending at
// endMs: each sung piece fills from its start to the next sung piece's. Pieces with no beats are done once the
// piece before them is.
export function timedSweep(pieces: readonly Piece[], starts: readonly number[], endMs: number, ms: number): number[] {
  const weights = pieces.map(pieceBeats);
  return pieces.map((_, j) => {
    const from = starts[j] ?? endMs;
    if ((weights[j] ?? 0) === 0) {
      return ms >= from ? 1 : 0;
    }
    const next = weights.findIndex((w, k) => k > j && w > 0);
    const to = next >= 0 ? (starts[next] ?? endMs) : endMs;
    if (to <= from) {
      return ms >= from ? 1 : 0;
    }
    return Math.min(1, Math.max(0, (ms - from) / (to - from)));
  });
}

// The sweep for line i of lines at lyric time ms, or null when there's none to show: no time to go by, or a line
// with no time. A line with word times sweeps by them, and its last word gets at least its beats at the song's
// pace before the line ends, though never past the next line's start.
export function lineSweep(
  lines: readonly SweptLine[],
  i: number,
  ms: number | null,
  beatMs: number = defaultBeatMs,
): number[] | null {
  const line = lines[i];
  if (ms === null || !line || line.startMs === null) {
    return null;
  }
  const pieces = line.pieces ?? [];
  const total = pieces.reduce((n, p) => n + pieceBeats(p), 0);
  let end = lineEnd(lines, i, total, beatMs);
  if (end === null) {
    return null;
  }
  const times = line.times ?? [];
  if (times.length === 0 || line.text === undefined) {
    return sweep(pieces, line.startMs, end, ms);
  }
  const offsets = pieceOffsets(line.text, pieces);
  const lastAt = Math.max(...times.map((t) => t.at));
  const lastMs = Math.max(...times.map((t) => t.ms));
  const after = pieces.reduce((n, p, j) => n + ((offsets[j] ?? 0) + 1 > lastAt ? pieceBeats(p) : 0), 0);
  end = Math.max(end, lastMs + Math.max(1, after) * beatMs);
  const next = nextStart(lines, i);
  if (next !== null) {
    end = Math.min(end, next);
  }
  return timedSweep(pieces, pieceStarts(pieces, offsets, times, line.startMs, end), end, ms);
}
