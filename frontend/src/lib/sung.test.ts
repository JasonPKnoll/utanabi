import { describe, expect, it } from "vitest";

import { finishSung, scriptOf, typeSung } from "./sung";

describe("typeSung", () => {
  it.each([
    ["konpasu", "katakana", "コンパス"],
    ["konpasu", "hiragana", "こんぱす"],
    ["ko-hi-", "katakana", "コーヒー"],
    ["コンパス", "hiragana", "こんぱす"],
    ["ほし", "katakana", "ホシ"],
  ] as const)("types %j in %s as %j", (raw, script, want) => {
    expect(typeSung(raw, script)).toBe(want);
  });

  it("waits on a lone n", () => {
    expect(typeSung("kon", "katakana")).toBe("コn");
  });
});

describe("finishSung", () => {
  it("turns a final n into ん and drops spaces", () => {
    expect(finishSung(" ほ し n ", "hiragana")).toBe("ほしん");
    expect(finishSung("コn", "katakana")).toBe("コン");
  });
});

describe("scriptOf", () => {
  it.each([
    ["コンパス", "katakana"],
    ["ほし", "hiragana"],
    ["", "katakana"],
  ] as const)("reads %j as %s", (reading, want) => {
    expect(scriptOf(reading)).toBe(want);
  });
});
