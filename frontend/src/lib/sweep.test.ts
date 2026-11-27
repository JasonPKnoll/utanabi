import { describe, expect, it } from "vitest";

import { beats, lineEnd, lineSweep, pieceBeats, songPace, sweep } from "./sweep";

describe("beats", () => {
  it.each([
    ["らしんばん", 5],
    ["きゃく", 2],
    ["コンパス", 4],
    ["ラーメン", 4],
    ["まって", 3],
    ["空", 2],
    ["、！ ", 0],
    ["yeah", 1],
    ["Sunday", 2],
    ["1番", 3],
  ])("%s is %d", (text, want) => {
    expect(beats(text)).toBe(want);
  });
});

describe("pieceBeats", () => {
  it("reads kanji by their reading", () => {
    expect(pieceBeats({ ruby: [{ text: "羅針盤", reading: "らしんばん" }] })).toBe(5);
    expect(
      pieceBeats({
        ruby: [
          { text: "探", reading: "さが" },
          { text: "して", reading: "" },
        ],
      }),
    ).toBe(4);
  });

  it("is 0 for no ruby", () => {
    expect(pieceBeats({ ruby: null })).toBe(0);
  });
});

describe("lineEnd", () => {
  const lines = [{ startMs: 1000 }, { startMs: null }, { startMs: 4000 }];

  it("ends a breath before the next timed line starts: 12% of the gap, at most 0.4 s", () => {
    expect(lineEnd(lines, 0, 10)).toBe(4000 - 360);
    expect(lineEnd([{ startMs: 1000 }, { startMs: 5000 }], 0, 40)).toBe(5000 - 400);
  });

  it("ends sooner when the gap is longer than its beats take at the song's pace, with some slack", () => {
    // 5 beats at 400 ms, the pace without a measured one, and 15% slack: 2.3 s.
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 5)).toBe(3300);
    // A fast song, at 200 ms a beat: 1.15 s.
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 5, 200)).toBe(2150);
  });

  it("takes at least a second", () => {
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 1)).toBe(2000);
  });

  it("ends by its beats when it's the last line", () => {
    expect(lineEnd(lines, 2, 5)).toBe(6300);
  });

  it("is null for a line with no time", () => {
    expect(lineEnd(lines, 1, 5)).toBeNull();
  });
});

describe("sweep", () => {
  // 夜 (よる, 2 beats), の (1), 空 (そら, 2): 5 beats over a second.
  const pieces = [
    { ruby: [{ text: "夜", reading: "よる" }] },
    { ruby: [{ text: "の", reading: "" }] },
    { ruby: [{ text: "、", reading: "" }] },
    { ruby: [{ text: "空", reading: "そら" }] },
  ];

  it.each([
    [0, [0, 0, 0, 0]],
    [1000, [0, 0, 0, 0]],
    [1200, [0.5, 0, 0, 0]],
    [1500, [1, 0.5, 0, 0]],
    [1600, [1, 1, 1, 0]],
    [1800, [1, 1, 1, 0.5]],
    [2000, [1, 1, 1, 1]],
    [9000, [1, 1, 1, 1]],
  ])("at %d ms", (ms, want) => {
    const got = sweep(pieces, 1000, 2000, ms);
    got.forEach((p, i) => expect(p).toBeCloseTo(want[i] ?? -1));
  });

  it("fills a line with no beats once it starts", () => {
    expect(sweep([{ ruby: [{ text: "！", reading: "" }] }], 1000, 2000, 1000)).toEqual([1]);
  });
});

describe("lineSweep", () => {
  const lines = [
    { startMs: 1000, ruby: null, pieces: [{ ruby: [{ text: "あい", reading: "" }] }] },
    { startMs: null, ruby: null, pieces: [{ ruby: [{ text: "うた", reading: "" }] }] },
  ];

  it("sweeps a timed line, the last one over its beats at the song's pace, but at least a second", () => {
    // Two beats at 400 ms with 15% slack is 0.92 s, so a second: 0.4 s in is 40%.
    expect(lineSweep(lines, 0, 1400)?.[0]).toBeCloseTo(0.4);
  });

  it("sweeps faster in a faster song", () => {
    // 2 beats at 200 ms with slack is 460 ms, but a line takes at least a second: 0.4 s in is 40%.
    expect(lineSweep(lines, 0, 1400, 200)?.[0]).toBeCloseTo(0.4);
    // Five beats over 1.15 s at 200 ms a beat: 0.575 s in is half.
    const five = [{ startMs: 1000, ruby: null, pieces: [{ ruby: [{ text: "あいうえお", reading: "" }] }] }];
    expect(lineSweep(five, 0, 1575, 200)?.[0]).toBeCloseTo(0.5);
  });

  it("is null with no time to go by, or for a line with no time", () => {
    expect(lineSweep(lines, 0, null)).toBeNull();
    expect(lineSweep(lines, 1, 1400)).toBeNull();
  });
});

describe("songPace", () => {
  // Lines of four beats each (あいうえ): most sung back to back at 200 ms a beat, some before a pause.
  const line = (startMs: number | null) => ({ startMs, pieces: [{ ruby: [{ text: "あいうえ", reading: "" }] }] });

  it("is the pace of the song's faster lines, not its pauses", () => {
    const lines = [0, 800, 1600, 2400, 3200, 8000, 8800, 9600, 10400, 16000].map(line);
    expect(songPace(lines)).toBe(200);
  });

  it("is 400 ms without enough lines to tell", () => {
    expect(songPace([line(0), line(800), line(1600)])).toBe(400);
    expect(songPace([])).toBe(400);
  });

  it("leaves out short lines and lines with no time", () => {
    const yeah = { startMs: 0, pieces: [{ ruby: [{ text: "yeah", reading: "" }] }] };
    const lines = [yeah, line(100), line(900), line(null), line(1700), line(2500), line(3300), line(4100)];
    expect(songPace(lines)).toBe(200);
  });

  it("stays between 100 and 600 ms a beat", () => {
    expect(songPace([0, 100, 200, 300, 400, 500].map(line))).toBe(100);
    expect(songPace([0, 4000, 8000, 12000, 16000, 20000].map(line))).toBe(600);
  });
});
