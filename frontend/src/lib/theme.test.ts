import { describe, expect, it } from "vitest";

import css from "../themes.css?raw";

import { loadMode, loadPalette, palettes, saveMode, savePalette } from "./theme";

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

describe("loadMode", () => {
  it("follows the device when nothing was chosen", () => {
    expect(loadMode(() => memoryStore())).toBe("auto");
  });

  it("reads back a saved mode", () => {
    const store = memoryStore();
    saveMode("light", () => store);
    expect(loadMode(() => store)).toBe("light");
  });

  it("ignores a value it doesn't know", () => {
    expect(loadMode(() => memoryStore({ "utanabi.theme": "sepia" }))).toBe("auto");
  });

  it("follows the device when storage refuses", () => {
    expect(loadMode(refusingStore)).toBe("auto");
  });
});

describe("loadPalette", () => {
  it("is Utanabi when nothing was chosen", () => {
    expect(loadPalette(() => memoryStore())).toBe("utanabi");
  });

  it("reads back a saved palette", () => {
    const store = memoryStore();
    savePalette("nord", () => store);
    expect(loadPalette(() => store)).toBe("nord");
  });

  it("ignores a palette that no longer exists", () => {
    expect(loadPalette(() => memoryStore({ "utanabi.palette": "dracula" }))).toBe("utanabi");
  });

  it("is Utanabi when storage refuses", () => {
    expect(loadPalette(refusingStore)).toBe("utanabi");
    expect(savePalette("vivid", refusingStore)).toBe(false);
  });
});

// themes.css, read as text: each palette's block, and in it each colour by name, such as d-bg or l-link.
function readPalettes(): Map<string, Map<string, string>> {
  const found = new Map<string, Map<string, string>>();
  for (const [, palette = "", body = ""] of css.matchAll(/\[data-palette="([a-z-]+)"\]\s*\{([^}]*)\}/g)) {
    const colours = new Map<string, string>();
    for (const [, name = "", value = ""] of body.matchAll(/--([dl]-[a-z-]+):\s*(#[0-9a-f]{6});/g)) {
      colours.set(name, value);
    }
    found.set(palette, colours);
  }
  return found;
}

function luminance(hex: string): number {
  const channel = (i: number) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
}

function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

const tokens = [
  "bg",
  "surface",
  "surface-strong",
  "border",
  "fg",
  "muted",
  "dim",
  "accent",
  "accent-strong",
  "on-accent",
  "link",
  "link-strong",
  "danger",
  "success",
  "spark",
];

// Text colours and the surfaces they're drawn on somewhere in the app.
const pairs: [string, string][] = [
  ...["fg", "muted", "link", "danger", "success"].flatMap((text) =>
    ["bg", "surface", "surface-strong"].map((on): [string, string] => [text, on]),
  ),
  ["dim", "bg"],
  ["dim", "surface"],
  ["on-accent", "accent"],
  ["on-accent", "accent-strong"],
];

describe("themes.css", () => {
  const found = readPalettes();

  it("has a block for every palette the picker offers, with every colour in both modes", () => {
    // Vitest blanks CSS unless vite.config.ts lets it through; an empty read would pass the contrast check below.
    expect([...found.keys()].sort()).toEqual(palettes.map((p) => p.value).sort());
    for (const { value } of palettes) {
      const colours = found.get(value);
      expect(colours, value).toBeDefined();
      for (const mode of ["d", "l"]) {
        for (const token of tokens) {
          expect(colours?.get(`${mode}-${token}`), `${value} ${mode}-${token}`).toMatch(/^#[0-9a-f]{6}$/);
        }
      }
    }
  });

  it("keeps every text colour at 4.5:1 or more on the surfaces it sits on", () => {
    const failures: string[] = [];
    for (const [palette, colours] of found) {
      for (const mode of ["d", "l"]) {
        for (const [text, on] of pairs) {
          const ratio = contrast(colours.get(`${mode}-${text}`) ?? "", colours.get(`${mode}-${on}`) ?? "");
          if (!(ratio >= 4.5)) {
            failures.push(`${palette} ${mode}: ${text} on ${on} is ${ratio.toFixed(2)}`);
          }
        }
      }
    }
    expect(failures).toEqual([]);
  });

  // The firework's colours are drawn on the palette's dark set in both modes, as graphics, so they need 3:1.
  it("keeps the firework's colours at 3:1 or more on the night sky", () => {
    const failures: string[] = [];
    for (const [palette, colours] of found) {
      for (const colour of ["accent", "accent-strong", "spark"]) {
        for (const on of ["bg", "surface"]) {
          const ratio = contrast(colours.get(`d-${colour}`) ?? "", colours.get(`d-${on}`) ?? "");
          if (!(ratio >= 3)) {
            failures.push(`${palette}: ${colour} on ${on} is ${ratio.toFixed(2)}`);
          }
        }
      }
      for (const on of ["bg", "surface"]) {
        const ratio = contrast(colours.get("l-spark") ?? "", colours.get(`l-${on}`) ?? "");
        if (!(ratio >= 3)) {
          failures.push(`${palette}: light spark on ${on} is ${ratio.toFixed(2)}`);
        }
      }
    }
    expect(failures).toEqual([]);
  });
});
