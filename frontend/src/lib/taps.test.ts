import { describe, expect, it } from "vitest";
import { tapAt, tappedAll, tapStops } from "./taps";

// 夜の、空 (three words to tap; the comma has none) from 1 s, then 君と (two) from 4 s.
const lines = [
  {
    startMs: 1000,
    ruby: null,
    text: "夜の、空",
    pieces: [
      { ruby: [{ text: "夜", reading: "よる" }] },
      { ruby: [{ text: "の", reading: "" }] },
      { ruby: [{ text: "、", reading: "" }] },
      { ruby: [{ text: "空", reading: "そら" }] },
    ],
  },
  {
    startMs: 4000,
    ruby: null,
    text: "君と",
    pieces: [{ ruby: [{ text: "君", reading: "きみ" }] }, { ruby: [{ text: "と", reading: "" }] }],
  },
];

describe("tapStops", () => {
  it("is where each sung word of a line starts", () => {
    expect(tapStops(lines[0]!)).toEqual([0, 1, 3]);
  });
});

describe("tapAt", () => {
  it("times the line being sung, one word a tap", () => {
    let tapping = tapAt(lines, null, 1100);
    expect(tapping).toEqual({ line: 0, times: [{ at: 0, ms: 1100 }] });
    tapping = tapAt(lines, tapping, 1600.4);
    expect(tapping).toEqual({
      line: 0,
      times: [
        { at: 0, ms: 1100 },
        { at: 1, ms: 1600 },
      ],
    });
    tapping = tapAt(lines, tapping, 2200);
    expect(tapping?.times.at(-1)).toEqual({ at: 3, ms: 2200 });
    expect(tappedAll(lines, tapping!)).toBe(true);
    // A tap past the line's last word, before the next line, times nothing.
    expect(tapAt(lines, tapping, 2500)).toBeNull();
  });

  it("starts the next line once it's being sung", () => {
    const tapping = tapAt(lines, { line: 0, times: [{ at: 0, ms: 1100 }] }, 4500);
    expect(tapping).toEqual({ line: 1, times: [{ at: 0, ms: 4500 }] });
  });

  it("finishes the line before just after the next starts, while it has words left", () => {
    const tapping = tapAt(
      lines,
      {
        line: 0,
        times: [
          { at: 0, ms: 1100 },
          { at: 1, ms: 1600 },
        ],
      },
      4100,
    );
    expect(tapping).toEqual({
      line: 0,
      times: [
        { at: 0, ms: 1100 },
        { at: 1, ms: 1600 },
        { at: 3, ms: 4100 },
      ],
    });
  });

  it("starts a line over when you come back to it", () => {
    const tapping = tapAt(lines, { line: 1, times: [{ at: 0, ms: 4500 }] }, 1200);
    expect(tapping).toEqual({ line: 0, times: [{ at: 0, ms: 1200 }] });
  });

  it("never goes back in time within a line", () => {
    expect(tapAt(lines, { line: 0, times: [{ at: 0, ms: 1500 }] }, 1400)?.times.at(-1)).toEqual({ at: 1, ms: 1500 });
  });

  it("times nothing before the first line", () => {
    expect(tapAt(lines, null, 500)).toBeNull();
  });
});
