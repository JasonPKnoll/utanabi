import { describe, expect, it } from "vitest";

import { WordKind } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
import { hasMeanings, lemmaLine, splitMeanings } from "./words";

const senses = [
  { index: 0, glosses: ["sky"], partsOfSpeech: ["noun"], notes: [] },
  { index: 1, glosses: ["weather"], partsOfSpeech: ["noun"], notes: [] },
  { index: 2, glosses: ["far-off place"], partsOfSpeech: ["noun"], notes: [] },
];

describe("splitMeanings", () => {
  it("puts the meaning used in the song first", () => {
    const { used, others } = splitMeanings(senses, { entryId: 7, senseIndex: 1 }, 7);
    expect(used?.glosses).toEqual(["weather"]);
    expect(others.map((s) => s.index)).toEqual([0, 2]);
  });

  it("has no used meaning when the word has none", () => {
    const { used, others } = splitMeanings(senses, { entryId: 0, senseIndex: null }, 7);
    expect(used).toBeNull();
    expect(others).toHaveLength(3);
  });

  it("has no used meaning when the sheet shows another entry", () => {
    expect(splitMeanings(senses, { entryId: 8, senseIndex: 0 }, 7).used).toBeNull();
  });
});

describe("hasMeanings", () => {
  it.each([
    [WordKind.WordContent, true],
    [WordKind.WordParticle, false],
    [WordKind.WordExpression, false],
    [WordKind.WordAuxiliary, false],
  ])("%s: %s", (kind, want) => {
    expect(hasMeanings({ kind })).toBe(want);
  });
});

describe("lemmaLine", () => {
  it.each([
    [{ lemma: "探す", lemmaReading: "さがす", lemmaRomaji: "sagasu" }, "探す · さがす · sagasu"],
    [{ lemma: "の", lemmaReading: "の", lemmaRomaji: "no" }, "の · no"],
    [{ lemma: "love", lemmaReading: "", lemmaRomaji: "" }, "love"],
  ])("%j is %j", (word, want) => {
    expect(lemmaLine(word)).toBe(want);
  });
});
