import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

// Whether the line at index i starts a new verse, so the song view can leave a gap before it.
export function startsStanza(lines: readonly Line[], i: number): boolean {
  const line = lines[i];
  const before = lines[i - 1];
  return line !== undefined && before !== undefined && line.stanza !== before.stanza;
}

// The first character of a title, for the square tile in the song list. Array.from keeps emoji and other
// characters outside the Basic Multilingual Plane whole.
export function initial(title: string): string {
  return Array.from(title.trim())[0] ?? "♪";
}
