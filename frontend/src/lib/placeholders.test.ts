import { describe, expect, it } from "vitest";

import { placeholderCards, placeholderLines } from "./placeholders";

describe("placeholderLines", () => {
  const lines = placeholderLines();

  it("is shaped like a song's lines, so the song view draws it the way it draws a song", () => {
    for (const line of lines) {
      const words = line.words ?? [];
      const pieces = line.pieces ?? [];
      expect(pieces.length).toBe(words.length);
      pieces.forEach((piece, i) => expect(piece.word).toBe(i));
      expect((line.ruby ?? []).map((p) => p.text).join("")).toBe(line.text);
      expect(line.romaji).not.toBe("");
    }
  });

  it("has two verses with different line lengths", () => {
    expect(new Set(lines.map((l) => l.stanza))).toEqual(new Set([0, 1]));
    expect(new Set(lines.map((l) => l.text.length)).size).toBeGreaterThan(3);
  });

  it("is the same every time", () => {
    expect(placeholderLines()).toEqual(lines);
  });
});

describe("placeholderCards", () => {
  it("has IDs no real card has", () => {
    expect(placeholderCards().every((c) => c.id < 0)).toBe(true);
  });
});
