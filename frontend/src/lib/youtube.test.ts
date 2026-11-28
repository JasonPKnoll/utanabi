import { describe, expect, it } from "vitest";

import { embedURL, errorNote } from "./youtube";

describe("embedURL", () => {
  it("is the no-cookie player, inline, driven from the page", () => {
    const url = new URL(embedURL("dQw4w9WgXcQ", "https://utanabi.example"));
    expect(url.origin + url.pathname).toBe("https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ");
    expect(url.searchParams.get("enablejsapi")).toBe("1");
    expect(url.searchParams.get("playsinline")).toBe("1");
    expect(url.searchParams.get("origin")).toBe("https://utanabi.example");
  });
});

describe("errorNote", () => {
  it.each([
    [150, "doesn't let it play outside YouTube"],
    [101, "doesn't let it play outside YouTube"],
    [100, "gone or private"],
    [153, "couldn't play"],
  ])("code %d says %s", (code, want) => {
    expect(errorNote(code)).toContain(want);
  });
});
