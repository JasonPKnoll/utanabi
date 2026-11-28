import { describe, expect, it } from "vitest";

import { lineParts, otherMeanings, placeOf, shownCards, startFilter } from "./studyList";

const card = (id: number, learned = false) => ({ id, progress: { learned } });
const cards = [card(1), card(2, true), card(3), card(4)];

describe("startFilter", () => {
  it("opens on the words to learn while there are some", () => {
    expect(startFilter(cards)).toBe("learn");
  });

  it("opens on all the words once every one is learned", () => {
    expect(startFilter([card(1, true), card(2, true)])).toBe("all");
  });

  it("opens on all the words of an empty deck", () => {
    expect(startFilter([])).toBe("all");
  });
});

describe("shownCards", () => {
  it("leaves out learned cards for learn", () => {
    expect(shownCards(cards, "learn").map((c) => c.id)).toEqual([1, 3, 4]);
  });

  it("keeps every card, in order, for all", () => {
    expect(shownCards(cards, "all").map((c) => c.id)).toEqual([1, 2, 3, 4]);
  });
});

describe("placeOf", () => {
  it.each([
    [1, { index: 0, previous: undefined, next: 2 }],
    [3, { index: 2, previous: 2, next: 4 }],
    [4, { index: 3, previous: 3, next: undefined }],
    [9, { index: -1 }],
  ])("places card %d", (id, want) => {
    expect(placeOf(cards, id)).toEqual(want);
  });
});

describe("lineParts", () => {
  it("splits a line around its word by characters", () => {
    expect(lineParts({ line: "𠮟られて泣いた", wordStart: 4, wordText: "泣い" })).toEqual(["𠮟られて", "泣い", "た"]);
  });

  it("keeps a word at the start of its line", () => {
    expect(lineParts({ line: "空が青い", wordStart: 0, wordText: "空" })).toEqual(["", "空", "が青い"]);
  });
});

describe("otherMeanings", () => {
  it("leaves out the meanings the word has in the song", () => {
    expect(otherMeanings("sky; heavens", ["sky", "heavens", "emptiness", "empty"])).toEqual(["emptiness", "empty"]);
  });

  it("keeps every meaning when the song's is unknown", () => {
    expect(otherMeanings("", ["sky", "empty"])).toEqual(["sky", "empty"]);
  });
});
