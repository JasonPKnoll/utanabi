import { describe, expect, it } from "vitest";

import { beats, lineEnd, lineSweep, pieceBeats, pieceOffsets, pieceStarts, songPace, sweep, timedSweep } from "./sweep";

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

  it("ends a breath before the next timed line starts: 12% of the gap, at most 0.7 s", () => {
    expect(lineEnd(lines, 0, 10)).toBe(4000 - 360);
    expect(lineEnd([{ startMs: 1000 }, { startMs: 10000 }], 0, 40)).toBe(10000 - 700);
  });

  it("ends sooner only when the gap is well past what its beats take at the song's pace", () => {
    // 5 beats at 400 ms, the pace without a measured one, 1.3 times over: 2.6 s.
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 5)).toBe(3600);
    // A fast song, at 200 ms a beat: 10 beats in 2.6 s.
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 10, 200)).toBe(3600);
  });

  it("runs to the next line when it's sung slower than the song's pace but not by much", () => {
    // 5 beats at 200 ms would be 1 s, but a line given 1.7 s is taken to be sung to a breath before the next.
    expect(lineEnd([{ startMs: 1000 }, { startMs: 2700 }], 0, 5, 200)).toBe(2700 - 204);
  });

  it("takes at least a second and a half", () => {
    expect(lineEnd([{ startMs: 1000 }, { startMs: 30000 }], 0, 1)).toBe(2500);
  });

  it("ends by its beats when it's the last line", () => {
    expect(lineEnd(lines, 2, 5)).toBe(6600);
  });

  it("is null for a line with no time", () => {
    expect(lineEnd(lines, 1, 5)).toBeNull();
  });
});

describe("sweep", () => {
  // 夜 (よる, 2 beats), の (1), 空 (そら, 2, and a beat more as the held last note): 6 beats over 0.6 s.
  const pieces = [
    { ruby: [{ text: "夜", reading: "よる" }] },
    { ruby: [{ text: "の", reading: "" }] },
    { ruby: [{ text: "、", reading: "" }] },
    { ruby: [{ text: "空", reading: "そら" }] },
  ];

  it.each([
    [0, [0, 0, 0, 0]],
    [1000, [0, 0, 0, 0]],
    [1100, [0.5, 0, 0, 0]],
    [1250, [1, 0.5, 0, 0]],
    [1300, [1, 1, 1, 0]],
    [1450, [1, 1, 1, 0.5]],
    [1600, [1, 1, 1, 1]],
    [9000, [1, 1, 1, 1]],
  ])("at %d ms", (ms, want) => {
    const got = sweep(pieces, 1000, 1600, ms);
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

  it("sweeps a timed line, the last one over its beats at the song's pace, but at least a second and a half", () => {
    // Two beats at 400 ms, 1.3 times over, is 1.04 s, so 1.5 s: 0.6 s in is 40%.
    expect(lineSweep(lines, 0, 1600)?.[0]).toBeCloseTo(0.4);
  });

  it("sweeps faster in a faster song", () => {
    // Ten beats over 2.6 s at 200 ms a beat: 1.3 s in is half.
    const ten = [{ startMs: 1000, ruby: null, pieces: [{ ruby: [{ text: "あいうえおかきくけこ", reading: "" }] }] }];
    expect(lineSweep(ten, 0, 2300, 200)?.[0]).toBeCloseTo(0.5);
    expect(lineSweep(ten, 0, 2300)?.[0]).toBeCloseTo(0.25);
  });

  it("is null with no time to go by, or for a line with no time", () => {
    expect(lineSweep(lines, 0, null)).toBeNull();
    expect(lineSweep(lines, 1, 1400)).toBeNull();
  });
});

describe("songPace", () => {
  // Lines of four beats each (あいうえ): most sung back to back at 200 ms a beat, some before a pause.
  const line = (startMs: number | null) => ({ startMs, pieces: [{ ruby: [{ text: "あいうえ", reading: "" }] }] });

  it("is the pace of the song's middle line, not its pauses", () => {
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

describe("word times", () => {
  // 夜 (よる, 2 beats), の (1), 、 (none), 空 (そら, 2, and a beat more held): the line 夜の、空.
  const pieces = [
    { ruby: [{ text: "夜", reading: "よる" }] },
    { ruby: [{ text: "の", reading: "" }] },
    { ruby: [{ text: "、", reading: "" }] },
    { ruby: [{ text: "空", reading: "そら" }] },
  ];

  it("finds where each piece starts in the line's text", () => {
    expect(pieceOffsets("夜の、空", pieces)).toEqual([0, 1, 2, 3]);
    // Text no piece covers, such as a space the analysis left out, is skipped over.
    expect(pieceOffsets("夜の 、空", pieces)).toEqual([0, 1, 3, 4]);
    // A piece that isn't in the text starts where the one before ended.
    expect(pieceOffsets("夜", pieces)).toEqual([0, 1, 2, 3]);
  });

  it("starts timed pieces at their times and spreads the rest by their beats", () => {
    const starts = pieceStarts(
      pieces,
      [0, 1, 2, 3],
      [
        { at: 0, ms: 1000 },
        { at: 3, ms: 1800 },
      ],
      1000,
      2400,
    );
    // の is beat 2 of the 3 between 夜 at 1000 and 空 at 1800.
    [1000, 1000 + (800 * 2) / 3, 1800, 1800].forEach((want, i) => expect(starts[i]).toBeCloseTo(want));
  });

  it("starts a line's first word when it's sung, not when the line starts", () => {
    const starts = pieceStarts(pieces, [0, 1, 2, 3], [{ at: 0, ms: 1500 }], 1000, 3000);
    expect(starts[0]).toBe(1500);
    // Without a time after it, the rest share the time to the line's end: の at beat 2 of 6.
    expect(starts[1]).toBeCloseTo(1500 + (1500 * 2) / 6);
  });

  it("never goes back in time, nor past the line's end", () => {
    const starts = pieceStarts(
      pieces,
      [0, 1, 2, 3],
      [
        { at: 0, ms: 1500 },
        { at: 3, ms: 1200 },
      ],
      1000,
      2000,
    );
    expect(starts[3]).toBe(1500);
    expect(pieceStarts(pieces, [0, 1, 2, 3], [{ at: 3, ms: 9000 }], 1000, 2000)[3]).toBe(2000);
  });

  it.each([
    [1400, [0.5, 0, 0, 0]],
    [1900, [1, 0.5, 0, 0]],
    [2000, [1, 1, 1, 0]],
    [2300, [1, 1, 1, 0.5]],
    [2600, [1, 1, 1, 1]],
  ])("sweeps each word from its start to the next's, at %d ms", (ms, want) => {
    const got = timedSweep(pieces, [1000, 1800, 2000, 2000], 2600, ms);
    got.forEach((p, i) => expect(p).toBeCloseTo(want[i] ?? -1));
  });

  it("sweeps a line with word times by them", () => {
    const lines = [
      {
        startMs: 1000,
        ruby: null,
        text: "夜の、空",
        pieces,
        times: [
          { at: 0, ms: 1000 },
          { at: 3, ms: 2000 },
        ],
      },
      { startMs: 4000, ruby: null, text: "次", pieces: [{ ruby: [{ text: "次", reading: "つぎ" }] }] },
    ];
    // の is beat 2 of the 3 between 夜 at 1000 and 空 at 2000: 夜 runs to 1667, so it's half sung at 1333.
    expect(lineSweep(lines, 0, 1000 + 1000 / 3, 400)?.[0]).toBeCloseTo(0.5);
    expect(lineSweep(lines, 0, 2000, 400)).toEqual([1, 1, 1, 0]);
  });

  it("gives a line's last timed word its beats before the line ends, but not past the next line", () => {
    // Three beats of 空 at 400 ms from 2000: 3.2 s, past the line's own end by its beats, and short of the next.
    const lines = [
      { startMs: 1000, ruby: null, text: "夜の、空", pieces, times: [{ at: 3, ms: 2000 }] },
      { startMs: 9000, ruby: null, text: "次", pieces: [] },
    ];
    expect(lineSweep(lines, 0, 2400, 400)?.[3]).toBeGreaterThan(0);
    expect(lineSweep(lines, 0, 2700, 400)?.[3]).toBeLessThan(1);
    const close = [lines[0]!, { startMs: 2500, ruby: null, text: "次", pieces: [] }];
    expect(lineSweep(close, 0, 2500, 400)?.[3]).toBe(1);
  });
});
