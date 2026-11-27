import { describe, expect, it } from "vitest";

import { forTrack, searchFor, timedForAnother } from "./versions";

describe("timedForAnother", () => {
  it.each([
    [191000, 185573, true],
    [186000, 185573, false],
    [0, 185573, false],
    [191000, 0, false],
  ])("lyrics %d long for a track %d long: %s", (lyrics, track, want) => {
    expect(timedForAnother(lyrics, track)).toBe(want);
  });
});

describe("forTrack", () => {
  const found = [
    { name: "the video's", synced: true, durationMs: 191000, songId: 12 },
    { name: "the single's", synced: true, durationMs: 186000, songId: 0 },
    { name: "plain text", synced: false, durationMs: 185000, songId: 0 },
    { name: "a close one", synced: true, durationMs: 185000, songId: 0 },
  ];

  it("is the synced lyrics closest to the track's length", () => {
    expect(forTrack(found, 185573, 12)?.name).toBe("the single's");
    expect(forTrack(found, 185100, 12)?.name).toBe("a close one");
  });

  it("is none when nothing is that long, or only the song itself is", () => {
    expect(forTrack(found, 200000, 12)).toBeNull();
    expect(forTrack([found[0]!], 191000, 12)).toBeNull();
  });
});

describe("searchFor", () => {
  it("puts the artist first unless the title names them", () => {
    expect(searchFor("Matsuri", "Fujii Kaze")).toBe("Fujii Kaze Matsuri");
    expect(searchFor("Fujii Kaze - Shinunoga E-Wa", "Fujii Kaze")).toBe("Fujii Kaze - Shinunoga E-Wa");
    expect(searchFor("夜に駆ける", "")).toBe("夜に駆ける");
  });
});
