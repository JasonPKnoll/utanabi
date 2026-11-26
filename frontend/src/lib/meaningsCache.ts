import {
  Service as Songs,
  type Meanings,
  type Word,
} from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

type WordKey = Pick<Word, "lemma" | "lemmaReading" | "entryId">;

export interface Lookups {
  entries: (ids: number[]) => Promise<Meanings[] | null>;
  word: (lemma: string, reading: string, entryId: number) => Promise<Meanings>;
}

// Meanings the app has already fetched, kept while it runs, so a word sheet opens without asking Go again. A song's
// meanings are fetched in one call as it opens (preload); a word it couldn't cover is fetched on its own the first
// time it's tapped, and kept too. Each map keeps at most limit items, dropping the oldest first.
export function createMeaningsCache(lookups: Lookups, limit = 5000) {
  const byEntry = new Map<number, Meanings>();
  const byWord = new Map<string, Promise<Meanings>>();
  // Entries a preload is fetching now, so a tap during it waits for that call instead of making its own.
  const loading = new Map<number, Promise<void>>();

  function keep<K, V>(map: Map<K, V>, key: K, value: V) {
    map.delete(key);
    map.set(key, value);
    if (map.size > limit) {
      const oldest = map.keys().next();
      if (!oldest.done) {
        map.delete(oldest.value);
      }
    }
  }

  const keyOf = (w: WordKey) => `${w.lemma}\u0000${w.lemmaReading}\u0000${w.entryId}`;

  function cached(word: WordKey): Meanings | undefined {
    return word.entryId > 0 ? byEntry.get(word.entryId) : undefined;
  }

  // Fetches, in one call, the meanings of the words that point at an entry not kept yet. A failure is dropped: each
  // sheet then asks for its own word.
  async function preload(words: readonly WordKey[]): Promise<void> {
    const ids = [...new Set(words.map((w) => w.entryId))].filter(
      (id) => id > 0 && !byEntry.has(id) && !loading.has(id),
    );
    if (ids.length === 0) {
      return;
    }
    const call = lookups.entries(ids).then(
      (list) => {
        for (const m of list ?? []) {
          keep(byEntry, m.entryId, m);
        }
      },
      () => undefined,
    );
    for (const id of ids) {
      loading.set(id, call);
    }
    try {
      await call;
    } finally {
      for (const id of ids) {
        loading.delete(id);
      }
    }
  }

  // The word's meanings: kept ones at once, otherwise from a preload already under way, otherwise asked for on their
  // own. A failed lookup isn't kept, so the next tap tries again.
  async function lookup(word: WordKey): Promise<Meanings> {
    const pending = word.entryId > 0 ? loading.get(word.entryId) : undefined;
    if (pending) {
      await pending;
    }
    const hit = cached(word);
    if (hit) {
      return hit;
    }
    const key = keyOf(word);
    let call = byWord.get(key);
    if (!call) {
      call = lookups.word(word.lemma, word.lemmaReading, word.entryId);
      keep(byWord, key, call);
      call.catch(() => byWord.delete(key));
    }
    return call;
  }

  return { cached, preload, lookup };
}

export const meaningsCache = createMeaningsCache({
  entries: (ids) => Songs.LookupEntries(ids),
  word: (lemma, reading, entryId) => Songs.LookupWord(lemma, reading, entryId),
});
