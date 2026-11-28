// Timing a song's words by tapping along: each tap is when the next word of the line being sung starts.
import { lineAt } from "./follow";
import { pieceBeats, pieceOffsets, type SweptLine, type WordTime } from "./sweep";

// The line being tapped, by its index in the song, and the times tapped for it so far.
export interface Tapping {
  line: number;
  times: WordTime[];
}

// Where each word you can tap starts in a line: its sung pieces, by the character each starts at. Punctuation and
// other pieces with nothing to sing get no tap.
export function tapStops(line: SweptLine): number[] {
  const pieces = line.pieces ?? [];
  const offsets = pieceOffsets(line.text ?? "", pieces);
  return pieces.flatMap((piece, j) => (pieceBeats(piece) > 0 ? [offsets[j] ?? 0] : []));
}

// How long after a line starts a tap may still finish the line before it: lyrics often start a line a moment before
// it's sung, while the last word of the line before still is.
const graceMs = 300;

// A tap at ms, the lyrics' time: the line it times and that line's times with it, or null when there's nothing left
// to time. A tap on a line other than the one being tapped starts that line over.
export function tapAt(lines: readonly SweptLine[], tapping: Tapping | null, ms: number): Tapping | null {
  let i = lineAt(lines, ms);
  const before = tapping ? lines[tapping.line] : undefined;
  if (
    tapping &&
    before &&
    i > tapping.line &&
    tapping.times.length < tapStops(before).length &&
    ms - (lines[i]?.startMs ?? 0) < graceMs
  ) {
    i = tapping.line;
  }
  const line = lines[i];
  if (i < 0 || !line) {
    return null;
  }
  const times = tapping?.line === i ? tapping.times : [];
  const stops = tapStops(line);
  const at = stops[times.length];
  if (at === undefined) {
    return null;
  }
  const last = times.at(-1)?.ms ?? 0;
  return { line: i, times: [...times, { at, ms: Math.round(Math.max(ms, last)) }] };
}

// Whether every word of the tapped line has its time.
export function tappedAll(lines: readonly SweptLine[], tapping: Tapping): boolean {
  const line = lines[tapping.line];
  return line !== undefined && tapping.times.length >= tapStops(line).length;
}
