import { describe, expect, it } from "vitest";

import { formatDuration, sourceLabel } from "./sources";

describe("sourceLabel", () => {
  it.each([
    ["user", "your lyrics"],
    ["lrclib", "LRCLIB"],
    ["qqmusic", "QQ Music"],
    ["somewhere", "somewhere"],
  ])("names %s as %s", (code, want) => {
    expect(sourceLabel(code)).toBe(want);
  });
});

describe("formatDuration", () => {
  it.each([
    [215500, "3:36"],
    [60000, "1:00"],
    [9000, "0:09"],
    [0, ""],
    [-5, ""],
  ])("shows %i ms as %j", (ms, want) => {
    expect(formatDuration(ms)).toBe(want);
  });
});
