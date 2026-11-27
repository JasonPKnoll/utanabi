import { describe, expect, it } from "vitest";

import { beats, lineEnd, lineSweep, pieceBeats, sweep } from "./sweep";

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

  it("ends when the next timed line starts", () => {
    expect(lineEnd(lines, 0, 10)).toBe(4000);
  });

  it("ends sooner when the gap is longer than its beats take at the slowest", () => {
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 5)).toBe(3000);
  });

  it("takes at least a second", () => {
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 1)).toBe(2000);
  });

  it("ends by its beats when it's the last line", () => {
    expect(lineEnd(lines, 2, 5)).toBe(6000);
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

  it("sweeps a timed line, the last one over at least a second", () => {
    expect(lineSweep(lines, 0, 1400)?.[0]).toBeCloseTo(0.4);
  });

  it("is null with no time to go by, or for a line with no time", () => {
    expect(lineSweep(lines, 0, null)).toBeNull();
    expect(lineSweep(lines, 1, 1400)).toBeNull();
  });
});
