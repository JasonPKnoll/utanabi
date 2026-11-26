import { describe, expect, it } from "vitest";

import { loadTheme, saveTheme } from "./theme";

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

describe("loadTheme", () => {
  it("follows the device when nothing was chosen", () => {
    expect(loadTheme(() => memoryStore())).toBe("auto");
  });

  it("reads back a saved theme", () => {
    const store = memoryStore();
    saveTheme("light", () => store);
    expect(loadTheme(() => store)).toBe("light");
  });

  it("ignores a value it doesn't know", () => {
    expect(loadTheme(() => memoryStore({ "utanabi.theme": "sepia" }))).toBe("auto");
  });

  it("follows the device when storage refuses", () => {
    expect(loadTheme(refusingStore)).toBe("auto");
  });
});

describe("saveTheme", () => {
  it("says when storage refuses", () => {
    expect(saveTheme("dark", refusingStore)).toBe(false);
  });
});
