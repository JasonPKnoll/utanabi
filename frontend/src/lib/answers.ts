import { toHiragana } from "wanakana";

// Checking typed answers on a flashcard, the way WaniKani does: a reading must match exactly once it's in hiragana,
// and a meaning may be any of the word's English meanings, ignoring case, punctuation, notes in parentheses, a leading
// "to", "be", "a", "an", or "the", and a small typo.

// The reading as typed, in hiragana: what the kana input made of it, with any leftover romaji (a final n) converted.
export function typedReading(answer: string): string {
  return toHiragana(answer.replace(/\s+/g, ""));
}

export function readingIsRight(answer: string, readings: readonly string[]): boolean {
  const typed = typedReading(answer);
  return typed !== "" && readings.some((r) => toHiragana(r) === typed);
}

// A meaning in its plain form: "To be (of animate objects)" and "be" both become "".
export function plainMeaning(text: string): string {
  let s = text
    .toLowerCase()
    .replace(/\([^)]*\)/g, " ")
    .replace(/[^\p{L}\p{N}' ]+/gu, " ")
    .replace(/\s+/g, " ")
    .trim();
  for (;;) {
    const shorter = s.replace(/^(to|be|a|an|the) /, "");
    if (shorter === s) {
      return s;
    }
    s = shorter;
  }
}

export function meaningIsRight(answer: string, meanings: readonly string[]): boolean {
  const typed = plainMeaning(answer);
  if (typed === "") {
    return false;
  }
  return meanings.some((m) => {
    const plain = plainMeaning(m);
    return plain !== "" && distance(typed, plain) <= typosAllowed(plain.length);
  });
}

// One typo in a word of four to seven letters, two in a longer one, none in a short word, where one letter changes
// the meaning.
function typosAllowed(length: number): number {
  if (length <= 3) {
    return 0;
  }
  return length <= 7 ? 1 : 2;
}

// The edit distance from a to b: how many letters to add, remove, change, or swap with the next one (Damerau's
// rule), so "shien" is one typo from "shine".
function distance(a: string, b: string): number {
  const x = [...a];
  const y = [...b];
  const d: number[][] = Array.from({ length: x.length + 1 }, (_, i) =>
    Array.from({ length: y.length + 1 }, (_, j) => (i === 0 ? j : j === 0 ? i : 0)),
  );
  const at = (i: number, j: number) => d[i]?.[j] ?? 0;
  for (let i = 1; i <= x.length; i++) {
    for (let j = 1; j <= y.length; j++) {
      const cost = x[i - 1] === y[j - 1] ? 0 : 1;
      let best = Math.min(at(i - 1, j) + 1, at(i, j - 1) + 1, at(i - 1, j - 1) + cost);
      if (i > 1 && j > 1 && x[i - 1] === y[j - 2] && x[i - 2] === y[j - 1]) {
        best = Math.min(best, at(i - 2, j - 2) + 1);
      }
      const row = d[i];
      if (row) {
        row[j] = best;
      }
    }
  }
  return at(x.length, y.length);
}
