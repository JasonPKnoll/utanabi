import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

// What the song view shows with each line: furigana over the kanji, and romaji under the line.
export interface ReadingAids {
  furigana: boolean;
  romaji: boolean;
}

export const defaultReadingAids: ReadingAids = { furigana: true, romaji: false };

const storageKey = "utanabi.readingAids";

type Store = Pick<Storage, "getItem" | "setItem">;

// The reading aids remembered on this device, or the defaults. Storage can be missing or refuse access (a private
// window, blocked site data), so every access is guarded and the defaults stand in.
export function loadReadingAids(store: () => Store = () => window.localStorage): ReadingAids {
  try {
    const raw = store().getItem(storageKey);
    if (raw === null) {
      return { ...defaultReadingAids };
    }
    const saved: unknown = JSON.parse(raw);
    if (typeof saved !== "object" || saved === null) {
      return { ...defaultReadingAids };
    }
    const { furigana, romaji } = saved as Partial<Record<keyof ReadingAids, unknown>>;
    return {
      furigana: typeof furigana === "boolean" ? furigana : defaultReadingAids.furigana,
      romaji: typeof romaji === "boolean" ? romaji : defaultReadingAids.romaji,
    };
  } catch {
    return { ...defaultReadingAids };
  }
}

// Remembers the reading aids on this device. It returns false when storage refuses, and the choice then lasts only
// until the screen closes.
export function saveReadingAids(aids: ReadingAids, store: () => Store = () => window.localStorage): boolean {
  try {
    store().setItem(storageKey, JSON.stringify(aids));
    return true;
  } catch {
    return false;
  }
}

// Whether a line's romaji adds anything: a line already in Latin letters reads the same.
export function hasRomaji(line: Line): boolean {
  return line.romaji !== "" && line.romaji !== line.text;
}
