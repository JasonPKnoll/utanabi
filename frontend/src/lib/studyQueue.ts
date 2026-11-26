// The order cards come round in a study session. It's a queue of card IDs: the card at the front is asked, then it
// goes to the back if you got it right and it isn't learned yet, comes back a few cards later if you got it wrong,
// and leaves the queue once it's learned.

export interface QueuedCard {
  id: number;
  learned: boolean;
}

// How many other cards come before a card you got wrong is asked again.
export const wrongComesBack = 3;

// A queue of the cards still to learn, or every card when all of them are learned, shuffled.
export function startQueue(cards: readonly QueuedCard[], all = false, random: () => number = Math.random): number[] {
  const ids = cards.filter((c) => all || !c.learned).map((c) => c.id);
  for (let i = ids.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [ids[i], ids[j]] = [ids[j] as number, ids[i] as number];
  }
  return ids;
}

// The queue after the card at its front was answered.
export function afterAnswer(queue: readonly number[], right: boolean, learned: boolean): number[] {
  const [card, ...rest] = queue;
  if (card === undefined || learned) {
    return rest;
  }
  if (right) {
    return [...rest, card];
  }
  const at = Math.min(wrongComesBack, rest.length);
  return [...rest.slice(0, at), card, ...rest.slice(at)];
}
