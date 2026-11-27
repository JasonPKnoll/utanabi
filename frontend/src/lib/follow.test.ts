import { describe, expect, it } from "vitest";

import { describeOffset, hasTimes, lineAt, lyricsTime, nextLine, positionAt, trackLink, trackTime } from "./follow";

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

describe("trackLink", () => {
  it("is the track's web link", () => {
    expect(trackLink("spotify:track:0kgczpcl7MmXUfXE4b0zVa")).toBe(
      "https://open.spotify.com/track/0kgczpcl7MmXUfXE4b0zVa",
    );
  });

  it.each(["", "spotify:album:0kgczpcl7MmXUfXE4b0zVa", "spotify:track:../x", "javascript:alert(1)"])(
    "is empty for %j",
    (uri) => {
      expect(trackLink(uri)).toBe("");
    },
  );
});

describe("offsets", () => {
  it("move a line's time onto the track, and back", () => {
    expect(trackTime(10000, -5500)).toBe(4500);
    expect(lyricsTime(4500, -5500)).toBe(10000);
    expect(trackTime(10000, 2000)).toBe(12000);
  });

  it("never put a line before the track starts", () => {
    expect(trackTime(1000, -5500)).toBe(0);
  });

  it.each([
    [0, "on time"],
    [40, "on time"],
    [-5500, "5.5 s earlier"],
    [2000, "2.0 s later"],
  ])("%d ms is %s", (offset, want) => {
    expect(describeOffset(offset)).toBe(want);
  });
});

describe("nextLine", () => {
  it.each([
    [0, 0],
    [1000, 1],
    [4500, 3],
    [9000, -1],
  ])("after %d ms is line %d", (ms, want) => {
    expect(nextLine(lines, ms)).toBe(want);
  });
});
