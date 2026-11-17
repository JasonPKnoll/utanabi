import { describe, expect, it } from "vitest";

import { defaultReadingAids, hasRomaji, loadReadingAids, saveReadingAids } from "./readingAids";

function memoryStore(initial: Record<string, string> = {}) {
  const items = new Map(Object.entries(initial));
  return {
    items,
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => void items.set(key, value),
  };
}

function refusingStore() {
  return {
    getItem: (): string | null => {
      throw new Error("SecurityError");
    },
    setItem: (): void => {
      throw new Error("QuotaExceededError");
    },
  };
}

describe("loadReadingAids", () => {
  it("starts with furigana on and romaji off", () => {
    expect(loadReadingAids(() => memoryStore())).toEqual({ furigana: true, romaji: false });
  });

  it("reads back what was saved", () => {
    const store = memoryStore();
    expect(saveReadingAids({ furigana: false, romaji: true }, () => store)).toBe(true);
    expect(loadReadingAids(() => store)).toEqual({ furigana: false, romaji: true });
  });

  it.each([
    ["not JSON", "{"],
    ["not an object", "true"],
    ["null", "null"],
    ["wrong types", '{"furigana":"yes","romaji":1}'],
  ])("falls back to the defaults for %s", (_, raw) => {
    expect(loadReadingAids(() => memoryStore({ "utanabi.readingAids": raw }))).toEqual(defaultReadingAids);
  });

  it("keeps a saved choice and defaults the one that's missing", () => {
    const store = memoryStore({ "utanabi.readingAids": '{"romaji":true}' });
    expect(loadReadingAids(() => store)).toEqual({ furigana: true, romaji: true });
  });

  it("falls back to the defaults when storage refuses", () => {
    expect(loadReadingAids(refusingStore)).toEqual(defaultReadingAids);
    expect(
      loadReadingAids(() => {
        throw new Error("no storage");
      }),
    ).toEqual(defaultReadingAids);
  });

  it("returns a copy, so changing it leaves the defaults alone", () => {
    loadReadingAids(() => memoryStore()).furigana = false;
    expect(defaultReadingAids.furigana).toBe(true);
  });
});

describe("saveReadingAids", () => {
  it("reports when storage refuses", () => {
    expect(saveReadingAids({ furigana: true, romaji: true }, refusingStore)).toBe(false);
  });
});

describe("hasRomaji", () => {
  const line = { position: 0, stanza: 0, startMs: null, ruby: null };

  it.each([
    ["夜の空", "yoru no sora", true],
    ["夜の空", "", false],
    ["love you", "love you", false],
  ])("%j with romaji %j: %s", (text, romaji, want) => {
    expect(hasRomaji({ ...line, text, romaji })).toBe(want);
  });
});
