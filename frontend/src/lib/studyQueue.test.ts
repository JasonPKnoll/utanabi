import { describe, expect, it } from "vitest";

import { afterAnswer, startQueue } from "./studyQueue";

const cards = [
  { id: 1, learned: false },
  { id: 2, learned: true },
  { id: 3, learned: false },
  { id: 4, learned: false },
];

describe("startQueue", () => {
  it("holds the cards still to learn", () => {
    expect(startQueue(cards).sort()).toEqual([1, 3, 4]);
  });

  it("holds every card when asked", () => {
    expect(startQueue(cards, true).sort()).toEqual([1, 2, 3, 4]);
  });

  it("shuffles them", () => {
    // With random always 0, each card swaps with the first, which reverses nothing in place but moves every card.
    expect(startQueue(cards, false, () => 0)).toEqual([3, 4, 1]);
  });
});

describe("afterAnswer", () => {
  it("sends a right card to the back", () => {
    expect(afterAnswer([1, 2, 3, 4, 5], true, false)).toEqual([2, 3, 4, 5, 1]);
  });

  it("brings a wrong card back after a few others", () => {
    expect(afterAnswer([1, 2, 3, 4, 5], false, false)).toEqual([2, 3, 4, 1, 5]);
  });

  it("brings a wrong card back at the end of a short queue", () => {
    expect(afterAnswer([1, 2], false, false)).toEqual([2, 1]);
  });

  it("drops a card once it's learned", () => {
    expect(afterAnswer([1, 2, 3], true, true)).toEqual([2, 3]);
  });

  it("asks the last card again until it's learned", () => {
    expect(afterAnswer([1], true, false)).toEqual([1]);
    expect(afterAnswer([1], true, true)).toEqual([]);
  });
});
