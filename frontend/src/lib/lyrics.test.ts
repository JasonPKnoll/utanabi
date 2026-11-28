import { describe, expect, it } from "vitest";

import { initial, startsStanza } from "./lyrics";

describe("startsStanza", () => {
  const lines = [
    {
      position: 0,
      stanza: 0,
      text: "夜の空に",
      startMs: null,
      ruby: null,
      romaji: "",
      words: null,
      pieces: null,
      gloss: "",
      times: null,
      timesBy: "",
    },
    {
      position: 1,
      stanza: 0,
      text: "星が光る",
      startMs: null,
      ruby: null,
      romaji: "",
      words: null,
      pieces: null,
      gloss: "",
      times: null,
      timesBy: "",
    },
    {
      position: 2,
      stanza: 1,
      text: "君の声を探している",
      startMs: null,
      ruby: null,
      romaji: "",
      words: null,
      pieces: null,
      gloss: "",
      times: null,
      timesBy: "",
    },
  ];

  it.each([
    [0, false],
    [1, false],
    [2, true],
    [3, false],
  ])("line %i starts a stanza: %s", (i, want) => {
    expect(startsStanza(lines, i)).toBe(want);
  });
});

describe("initial", () => {
  it.each([
    ["星の歌", "星"],
    ["  spring", "s"],
    ["🎤 live", "🎤"],
    ["", "♪"],
  ])("of %j is %j", (title, want) => {
    expect(initial(title)).toBe(want);
  });
});
