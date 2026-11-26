import { describe, expect, it } from "vitest";

import type { Meanings } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
import { createMeaningsCache, type Lookups } from "./meaningsCache";

function meanings(entryId: number, gloss = `gloss ${entryId}`): Meanings {
  return {
    installed: true,
    entryId,
    senses: [{ index: 0, glosses: [gloss], partsOfSpeech: [], notes: [] }],
    attribution: "JMdict",
  };
}

function fakeLookups(overrides: Partial<Lookups> = {}) {
  const calls = { entries: [] as number[][], word: [] as string[] };
  const lookups: Lookups = {
    entries: async (ids) => {
      calls.entries.push(ids);
      return ids.map((id) => meanings(id));
    },
    word: async (lemma, _reading, entryId) => {
      calls.word.push(lemma);
      return meanings(entryId, lemma);
    },
    ...overrides,
  };
  return { calls, lookups };
}

const star = { lemma: "星", lemmaReading: "ほし", entryId: 1376310 };
const sky = { lemma: "空", lemmaReading: "そら", entryId: 1245290 };
const unknown = { lemma: "げんまん", lemmaReading: "げんまん", entryId: 0 };

describe("meaningsCache", () => {
  it("preloads a song's entries in one call and answers from it", async () => {
    const { calls, lookups } = fakeLookups();
    const cache = createMeaningsCache(lookups);

    await cache.preload([star, sky, star, unknown]);
    expect(calls.entries).toEqual([[1376310, 1245290]]);
    expect(cache.cached(star)?.entryId).toBe(1376310);
    expect((await cache.lookup(sky)).entryId).toBe(1245290);
    expect(calls.word).toEqual([]);
  });

  it("doesn't fetch entries it already has", async () => {
    const { calls, lookups } = fakeLookups();
    const cache = createMeaningsCache(lookups);

    await cache.preload([star]);
    await cache.preload([star, sky]);
    expect(calls.entries).toEqual([[1376310], [1245290]]);
  });

  it("looks up a word with no entry once, then keeps it", async () => {
    const { calls, lookups } = fakeLookups();
    const cache = createMeaningsCache(lookups);

    expect(cache.cached(unknown)).toBeUndefined();
    await cache.lookup(unknown);
    await cache.lookup(unknown);
    expect(calls.word).toEqual(["げんまん"]);
  });

  it("waits for a preload under way instead of asking again", async () => {
    let finish: (list: Meanings[]) => void = () => {};
    const { calls, lookups } = fakeLookups({
      entries: (ids) => {
        calls.entries.push(ids);
        return new Promise((resolve) => (finish = resolve));
      },
    });
    const cache = createMeaningsCache(lookups);

    const preload = cache.preload([star]);
    const tapped = cache.lookup(star);
    finish([meanings(1376310)]);
    expect((await tapped).entryId).toBe(1376310);
    await preload;
    expect(calls.word).toEqual([]);
  });

  it("falls back to its own lookup when the preload fails", async () => {
    const { calls, lookups } = fakeLookups({ entries: () => Promise.reject(new Error("offline")) });
    const cache = createMeaningsCache(lookups);

    await cache.preload([star]);
    expect((await cache.lookup(star)).entryId).toBe(1376310);
    expect(calls.word).toEqual(["星"]);
  });

  it("tries again after a failed lookup", async () => {
    let fail = true;
    const { calls, lookups } = fakeLookups({
      word: async (lemma, _reading, entryId) => {
        calls.word.push(lemma);
        if (fail) {
          throw new Error("offline");
        }
        return meanings(entryId, lemma);
      },
    });
    const cache = createMeaningsCache(lookups);

    await expect(cache.lookup(unknown)).rejects.toThrow("offline");
    fail = false;
    await cache.lookup(unknown);
    expect(calls.word).toEqual(["げんまん", "げんまん"]);
  });

  it("drops the oldest meanings past its limit", async () => {
    const { lookups } = fakeLookups();
    const cache = createMeaningsCache(lookups, 2);

    await cache.preload([
      { ...star, entryId: 1 },
      { ...star, entryId: 2 },
      { ...star, entryId: 3 },
    ]);
    expect(cache.cached({ ...star, entryId: 1 })).toBeUndefined();
    expect(cache.cached({ ...star, entryId: 3 })).toBeDefined();
  });
});
