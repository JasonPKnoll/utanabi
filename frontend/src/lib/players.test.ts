import { describe, expect, it } from "vitest";

import { loadPlayer, playerFor, savePlayer } from "./players";

function memory(): Pick<Storage, "getItem" | "setItem"> {
  const kept = new Map<string, string>();
  return { getItem: (k) => kept.get(k) ?? null, setItem: (k, v) => void kept.set(k, v) };
}

describe("the player choice", () => {
  it("is Spotify until YouTube is chosen, and remembered", () => {
    const store = memory();
    expect(loadPlayer(() => store)).toBe("spotify");
    expect(savePlayer("youtube", () => store)).toBe(true);
    expect(loadPlayer(() => store)).toBe("youtube");
  });

  it("is Spotify when storage refuses", () => {
    const refusing = () => {
      throw new Error("blocked");
    };
    expect(loadPlayer(refusing)).toBe("spotify");
    expect(savePlayer("youtube", refusing)).toBe(false);
  });
});

describe("playerFor", () => {
  it.each([
    ["youtube", ["spotify", "youtube"], "youtube"],
    ["spotify", ["youtube"], "youtube"],
    ["youtube", ["spotify"], "spotify"],
    ["spotify", [], null],
  ] as const)("chosen %s with %j is %s", (chosen, available, want) => {
    expect(playerFor(chosen, available)).toBe(want);
  });
});
