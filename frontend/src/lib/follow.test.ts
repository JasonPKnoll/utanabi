import { describe, expect, it } from "vitest";

import {
  answeredAt,
  caughtUp,
  clockAt,
  describeOffset,
  hasTimes,
  lineAt,
  lyricsTime,
  nextLine,
  positionAt,
  steer,
  trackLink,
  trackTime,
} from "./follow";

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

describe("caughtUp", () => {
  const jump = { trackMs: 30000, playing: true, at: 1000 };

  it.each([
    ["Spotify still at the old place", { progressMs: 92000, playing: true, matches: true }, 1800, false],
    ["about where the jump put it by now", { progressMs: 31200, playing: true, matches: true }, 2000, true],
    ["paused when it should play", { progressMs: 30500, playing: false, matches: true }, 1500, false],
    ["still on the song before", { progressMs: 30500, playing: true, matches: false }, 1500, false],
    ["anything, once the move is old", { progressMs: 92000, playing: true, matches: true }, 6500, true],
  ])("%s", (_, answer, now, want) => {
    expect(caughtUp(jump, answer, now)).toBe(want);
  });

  it("holds a pause where it was made", () => {
    const pause = { trackMs: 45000, playing: false, at: 1000 };
    expect(caughtUp(pause, { progressMs: 45200, playing: false, matches: true }, 4000)).toBe(true);
    expect(caughtUp(pause, { progressMs: 47000, playing: true, matches: true }, 3000)).toBe(false);
  });
});

describe("the clock", () => {
  const running = { at: 1000, ms: 60000, rate: 1, playing: true };

  it("counts on while playing, and stays put while paused", () => {
    expect(clockAt(running, 3000)).toBe(62000);
    expect(clockAt({ ...running, playing: false }, 3000)).toBe(60000);
  });

  it("never steps back for an answer a little behind it", () => {
    // At 3000 the clock says 62000; Spotify says 61700.
    const steered = steer(running, 61700, true, 3000);
    expect(clockAt(steered, 3000)).toBe(62000);
    expect(steered.rate).toBeLessThan(1);
    expect(steered.rate).toBeGreaterThan(0);
    // Two seconds on, it has met Spotify, which played on at 1.
    expect(clockAt(steered, 5000)).toBeCloseTo(63700, -1);
  });

  it("speeds up for an answer a little ahead", () => {
    const steered = steer(running, 62400, true, 3000);
    expect(clockAt(steered, 3000)).toBe(62000);
    expect(steered.rate).toBeGreaterThan(1);
  });

  it("jumps for an answer far off, as after a seek", () => {
    expect(steer(running, 90000, true, 3000)).toEqual({ at: 3000, ms: 90000, rate: 1, playing: true });
  });

  it("is set exactly when paused, or with nothing to steer from", () => {
    expect(steer(running, 61700, false, 3000)).toEqual({ at: 3000, ms: 61700, rate: 1, playing: false });
    expect(steer(null, 5000, true, 3000)).toEqual({ at: 3000, ms: 5000, rate: 1, playing: true });
  });
});

describe("answeredAt", () => {
  it("takes the position as of halfway through the round trip", () => {
    expect(answeredAt(60000, true, 1000, 1400)).toBe(60200);
  });

  it("is the position as it is while paused", () => {
    expect(answeredAt(60000, false, 1000, 1400)).toBe(60000);
  });
});
