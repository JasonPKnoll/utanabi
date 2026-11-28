import { describe, expect, it } from "vitest";
import { lineEnd, lineSweep, songPace, type Piece } from "./sweep";
import real from "./testdata/real-timings.json";

// Scores the sweep against real singing: Japanese songs timed character by character by hand (the AMLL TTML DB,
// rebuilt with scripts/real-timings.ts). The sweep is given only each line's start, as line-synced lyrics give it, and
// each character's estimated start is compared with when it's really sung. Readings come from a rough count (a kanji
// is two beats), not the app's analysis.

interface Line {
  startMs: number;
  ruby: null;
  pieces: Piece[];
  endMs: number;
  onsetsMs: number[];
}

const songs: Line[][] = real.songs.map((song) =>
  song.lines.map((line) => ({
    startMs: line.onsetsMs[0] ?? 0,
    ruby: null,
    pieces: line.beats.map((n) => ({ ruby: [{ text: "", reading: "あ".repeat(n) }] })),
    endMs: line.endMs,
    onsetsMs: line.onsetsMs,
  })),
);

// When piece j of line i first starts to fill, found by halving the line's time.
function estimatedStart(lines: Line[], i: number, j: number, end: number, pace: number): number {
  let lo = lines[i]?.startMs ?? 0;
  let hi = end;
  while (hi - lo > 1) {
    const mid = (lo + hi) / 2;
    if ((lineSweep(lines, i, mid, pace)?.[j] ?? 0) > 0) {
      hi = mid;
    } else {
      lo = mid;
    }
  }
  return lo;
}

interface Score {
  meanMs: number;
  late: number;
  early: number;
}

// The mean error of each character's start, and the share of lines whose sweep ends over a second after or before
// the singing does. A song's last line has nothing to end it, so it's left out.
function score(estimate: (lines: Line[], i: number, j: number) => number, end: (lines: Line[], i: number) => number) {
  let error = 0;
  let chars = 0;
  let late = 0;
  let early = 0;
  let lines = 0;
  for (const song of songs) {
    song.slice(0, -1).forEach((line, i) => {
      line.onsetsMs.forEach((onset, j) => {
        error += Math.abs(estimate(song, i, j) - onset);
        chars++;
      });
      const off = end(song, i) - line.endMs;
      late += off > 1000 ? 1 : 0;
      early += off < -1000 ? 1 : 0;
      lines++;
    });
  }
  return { meanMs: error / chars, late: late / lines, early: early / lines } satisfies Score;
}

const total = (line: Line) => line.pieces.reduce((n, p) => n + (p.ruby?.[0]?.reading.length ?? 0), 0);

function ours(): Score {
  const paces = new Map(songs.map((song) => [song, songPace(song)]));
  const end = (song: Line[], i: number) => lineEnd(song, i, total(song[i]!), paces.get(song)) ?? 0;
  return score((song, i, j) => estimatedStart(song, i, j, end(song, i), paces.get(song) ?? 400), end);
}

// Sonora's sweep for line-synced lyrics: each character an even share of the time until the next line starts.
function evenToNextLine(): Score {
  const next = (song: Line[], i: number) => song[i + 1]?.startMs ?? 0;
  return score((song, i, j) => {
    const line = song[i]!;
    return line.startMs + ((next(song, i) - line.startMs) * j) / line.onsetsMs.length;
  }, next);
}

describe("the sweep against real singing", () => {
  const got = ours();

  it("has enough songs to go by", () => {
    expect(songs.length).toBeGreaterThanOrEqual(30);
  });

  it("starts each character within about a third of a second of the singing, on average", () => {
    expect(got.meanMs).toBeLessThan(340);
  });

  it("rarely runs on after the singing stops, or ends well before it", () => {
    expect(got.late).toBeLessThan(0.07);
    expect(got.early).toBeLessThan(0.08);
  });

  it("is closer to the singing than an even split to the next line", () => {
    const even = evenToNextLine();
    expect(got.meanMs).toBeLessThan(even.meanMs);
    expect(got.late).toBeLessThan(even.late);
  });
});
