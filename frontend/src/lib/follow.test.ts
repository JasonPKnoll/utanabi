import { describe, expect, it } from "vitest";

import { hasTimes, lineAt, positionAt } from "./follow";

const lines = [{ startMs: 1000 }, { startMs: 4500 }, { startMs: null }, { startMs: 9000 }];

describe("lineAt", () => {
  it.each([
    [0, -1],
    [999, -1],
    [1000, 0],
    [4499, 0],
    [4500, 1],
    [8999, 1],
    [9000, 3],
    [500000, 3],
  ])("at %d ms is line %d", (ms, want) => {
    expect(lineAt(lines, ms)).toBe(want);
  });

  it("is -1 for lyrics with no times", () => {
    expect(lineAt([{ startMs: null }, { startMs: null }], 5000)).toBe(-1);
  });
});

describe("hasTimes", () => {
  it("is true for synced lyrics only", () => {
    expect(hasTimes(lines)).toBe(true);
    expect(hasTimes([{ startMs: null }])).toBe(false);
  });
});

describe("positionAt", () => {
  it("counts on from Spotify's last answer while playing", () => {
    expect(positionAt(60000, true, 1000, 2500)).toBe(61500);
  });

  it("stays put while paused", () => {
    expect(positionAt(60000, false, 1000, 9000)).toBe(60000);
  });

  it("never counts back, if the clock does", () => {
    expect(positionAt(60000, true, 2000, 1500)).toBe(60000);
  });
});
