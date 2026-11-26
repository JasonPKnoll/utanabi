import { describe, expect, it } from "vitest";

import { meaningIsRight, plainMeaning, readingIsRight, typedReading } from "./answers";

describe("readingIsRight", () => {
  it.each([
    ["ひかる", ["ひかる"], true],
    ["ヒカル", ["ひかる"], true],
    ["hikaru", ["ひかる"], true],
    ["ほn", ["ほん"], true],
    ["ついたち", ["いちにち", "ついたち"], true],
    [" ひかる ", ["ひかる"], true],
    ["ひかり", ["ひかる"], false],
    ["", ["ひかる"], false],
  ])("%j for %j is %s", (answer, readings, want) => {
    expect(readingIsRight(answer, readings)).toBe(want);
  });

  it("finishes a trailing n the kana input left as romaji", () => {
    expect(typedReading("ほn")).toBe("ほん");
  });
});

describe("plainMeaning", () => {
  it.each([
    ["To shine", "shine"],
    ["to be fickle", "fickle"],
    ["pinky swear (i.e. linking little fingers to confirm a promise)", "pinky swear"],
    ["star (usu. excluding the Sun)", "star"],
    ["a mirror", "mirror"],
    ["(not) any more", "any more"],
    ["   who? ", "who"],
  ])("%j is %j", (text, want) => {
    expect(plainMeaning(text)).toBe(want);
  });
});

describe("meaningIsRight", () => {
  const shine = ["to shine", "to glitter", "to stand out"];

  it.each([
    ["shine", shine, true],
    ["to glitter", shine, true],
    ["Stand out!", shine, true],
    ["shien", shine, true],
    ["glimmer", shine, false],
    ["pinky swear", ["pinky swear (i.e. linking little fingers to confirm a promise)"], true],
    ["fickle", ["to be fickle"], true],
    ["who", ["who"], true],
    ["why", ["who"], false],
    ["", shine, false],
  ])("%j for %j is %s", (answer, meanings, want) => {
    expect(meaningIsRight(answer, meanings)).toBe(want);
  });
});
