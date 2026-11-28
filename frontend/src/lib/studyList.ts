// Which of a song's word cards the study list shows, and how a card steps to its neighbours in it.

export interface ListedCard {
  id: number;
  progress: { learned: boolean };
}

// "learn" shows the cards not learned yet, "all" every card.
export type Filter = "learn" | "all";

// The list opens on the words still to learn, or on all of them once every one is learned.
export function startFilter(cards: readonly ListedCard[]): Filter {
  return cards.some((c) => !c.progress.learned) ? "learn" : "all";
}

export function shownCards<T extends ListedCard>(cards: readonly T[], filter: Filter): T[] {
  return filter === "all" ? [...cards] : cards.filter((c) => !c.progress.learned);
}

export interface Place {
  // The card's place in the list, from 0, or -1 when it isn't in it.
  index: number;
  previous?: number;
  next?: number;
}

// Where a card is in the list, and the IDs of the cards before and after it.
export function placeOf(list: readonly ListedCard[], id: number): Place {
  const index = list.findIndex((c) => c.id === id);
  if (index < 0) {
    return { index };
  }
  return { index, previous: list[index - 1]?.id, next: list[index + 1]?.id };
}

export interface SungIn {
  line: string;
  wordStart: number;
  wordText: string;
}

// A card's line split around its word, by characters (wordStart counts runes, as Go does), for marking the word.
export function lineParts(c: SungIn): [string, string, string] {
  const chars = Array.from(c.line);
  const end = c.wordStart + Array.from(c.wordText).length;
  return [chars.slice(0, c.wordStart).join(""), chars.slice(c.wordStart, end).join(""), chars.slice(end).join("")];
}

// A card's other meanings: the ones the word has outside the song, without the song's own.
export function otherMeanings(meaning: string, meanings: readonly string[]): string[] {
  const own = new Set(meaning.split("; "));
  return meanings.filter((m) => !own.has(m));
}
