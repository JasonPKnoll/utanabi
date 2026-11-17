import { describe, expect, it } from "vitest";

import { firstSungLine, initial, lyricLines } from "./lyrics";

describe("lyricLines", () => {
  it("keeps blank lines between verses", () => {
    expect(lyricLines("夜の空に\n\n星が光る  ")).toEqual(["夜の空に", "", "星が光る"]);
  });
});

describe("firstSungLine", () => {
  it("skips leading blank lines", () => {
    expect(firstSungLine(["", "", "星が光る"])).toBe(2);
  });

  it("falls back to the first line when every line is blank", () => {
    expect(firstSungLine(["", ""])).toBe(0);
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
