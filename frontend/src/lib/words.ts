import {
  WordKind,
  type Meaning,
  type Word,
} from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

// The meanings for the word sheet: the one the word has in this song first, then the others in dictionary order.
// used is null when the song's word has no meaning chosen, or points at another entry.
export function splitMeanings(
  senses: readonly Meaning[],
  word: Pick<Word, "entryId" | "senseIndex">,
  entryId: number,
): { used: Meaning | null; others: Meaning[] } {
  const usedIndex = word.entryId !== 0 && word.entryId === entryId ? word.senseIndex : null;
  const used = senses.find((s) => s.index === usedIndex) ?? null;
  return { used, others: senses.filter((s) => s !== used) };
}

// Whether the sheet should look the word up in the dictionary: content words have meanings; particles, set phrases,
// and grammar words are explained by their grammar point.
export function hasMeanings(word: Pick<Word, "kind">): boolean {
  return word.kind === WordKind.WordContent;
}

// The dictionary form and its reading, for the sheet's second line: "探す · さがす · sagasu". Parts that are empty, or
// that repeat the form, are left out.
export function lemmaLine(word: Pick<Word, "lemma" | "lemmaReading" | "lemmaRomaji">): string {
  const parts = [word.lemma];
  if (word.lemmaReading && word.lemmaReading !== word.lemma) {
    parts.push(word.lemmaReading);
  }
  if (word.lemmaRomaji) {
    parts.push(word.lemmaRomaji);
  }
  return parts.join(" · ");
}
