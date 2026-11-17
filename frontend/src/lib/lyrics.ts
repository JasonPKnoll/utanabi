// Splits lyrics into lines. Blank lines stay, as empty strings, because they separate verses.
export function lyricLines(lyrics: string): string[] {
  return lyrics.split("\n").map((line) => line.trimEnd());
}

// The index of the first line with words, so a song opens on its first sung line.
export function firstSungLine(lines: string[]): number {
  return Math.max(
    0,
    lines.findIndex((line) => line !== ""),
  );
}

// The first character of a title, for the square tile in the song list. Array.from keeps emoji and other
// characters outside the Basic Multilingual Plane whole.
export function initial(title: string): string {
  return Array.from(title.trim())[0] ?? "♪";
}
