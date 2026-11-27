// Which player a song's lyrics follow: Spotify, playing on your Spotify or in the page, or YouTube, a song's video
// playing in the page. The choice is remembered on this device; one that isn't there to use falls back to the other.

export type Player = "spotify" | "youtube";

const storageKey = "utanabi.player";

type Store = Pick<Storage, "getItem" | "setItem">;

export function loadPlayer(store: () => Store = () => window.localStorage): Player {
  try {
    return store().getItem(storageKey) === "youtube" ? "youtube" : "spotify";
  } catch {
    return "spotify";
  }
}

// Remembers the choice on this device. It returns false when storage refuses, and the choice then lasts only until
// the screen closes.
export function savePlayer(player: Player, store: () => Store = () => window.localStorage): boolean {
  try {
    store().setItem(storageKey, player);
    return true;
  } catch {
    return false;
  }
}

// The player to use: the one chosen when it's there, else whichever is, else none.
export function playerFor(chosen: Player, available: readonly Player[]): Player | null {
  if (available.includes(chosen)) {
    return chosen;
  }
  return available[0] ?? null;
}
