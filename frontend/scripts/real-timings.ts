// Builds src/lib/testdata/real-timings.json, the word timings sweep.real.test.ts scores the sweep against: Japanese
// songs from the AMLL TTML DB (CC0), timed character by character by hand. It keeps only times and beat counts, never
// the lyrics, so no song's words are copied into the repo. Run it from frontend/ with:
//
//   node --experimental-strip-types scripts/real-timings.ts
import { writeFileSync } from "node:fs";
import { beats } from "../src/lib/sweep.ts";

// The database at a fixed commit, so the test data can be rebuilt exactly.
const commit = "533e9c52796db48f8bbde4394aee2b7d9db5d068";
const base = `https://raw.githubusercontent.com/amll-dev/amll-ttml-db/${commit}`;
const out = new URL("../src/lib/testdata/real-timings.json", import.meta.url);
// Every 15th Japanese song in the database, at most 60 of them.
const every = 15;
const most = 60;

interface Char {
  text: string;
  begin: number;
  end: number;
}

function ms(clock: string): number {
  const parts = clock.split(":").map(Number);
  const seconds = parts.reduce((total, part) => total * 60 + part, 0);
  return Math.round(seconds * 1000);
}

// The lead vocal's lines, each a list of its timed characters: translations, romaji, and backing vocals left out.
function parse(ttml: string): Char[][] {
  const lines: Char[][] = [];
  for (const p of ttml.matchAll(/<p [^>]*>([\s\S]*?)<\/p>/g)) {
    const body = (p[1] ?? "")
      .replace(/<span[^>]*ttm:role="x-(?:translation|roman)"[^>]*>[\s\S]*?<\/span>/g, "")
      .replace(/<span[^>]*ttm:role="x-bg"[^>]*>[\s\S]*?<\/span>\s*<\/span>/g, "");
    const chars: Char[] = [];
    for (const s of body.matchAll(/<span begin="([^"]+)" end="([^"]+)">([^<]*)<\/span>/g)) {
      const text = s[3] ?? "";
      if (text.trim()) {
        chars.push({ text, begin: ms(s[1] ?? "0"), end: ms(s[2] ?? "0") });
      }
    }
    if (chars.length > 0) {
      lines.push(chars);
    }
  }
  return lines;
}

// A song worth scoring: enough lines, timed by the character rather than the line, and mostly Japanese.
function usable(lines: Char[][]): boolean {
  const chars = lines.flat();
  const japanese = chars.filter((c) => /[ぁ-んァ-ン一-鿿]/.test(c.text)).length;
  return (
    lines.length >= 8 && lines.filter((l) => l.length > 1).length / lines.length > 0.8 && japanese / chars.length > 0.6
  );
}

async function text(path: string): Promise<string> {
  const res = await fetch(`${base}/${path}`);
  if (!res.ok) {
    throw new Error(`real-timings: can't fetch ${path} (status ${res.status})`);
  }
  return res.text();
}

const index = (await text("metadata/raw-lyrics-index.jsonl"))
  .split("\n")
  .filter((row) => row.trim())
  .map((row) => JSON.parse(row) as { metadata: [string, string[]][]; rawLyricFile: string });
const seen = new Set<string>();
const files: string[] = [];
for (const row of [...index].sort((a, b) => a.rawLyricFile.localeCompare(b.rawLyricFile))) {
  const meta = new Map(row.metadata);
  const song = `${(meta.get("musicName") ?? []).join(" ")}|${(meta.get("artists") ?? []).join(" ")}`;
  if (/[ぁ-んァ-ン]/.test(song) && !seen.has(song)) {
    seen.add(song);
    files.push(row.rawLyricFile);
  }
}

const songs = [];
for (const file of files.filter((_, i) => i % every === 0)) {
  const lines = parse(await text(`raw-lyrics/${file}`));
  if (!usable(lines)) {
    continue;
  }
  songs.push({
    file,
    lines: lines.map((chars) => ({
      endMs: chars[chars.length - 1]?.end ?? 0,
      beats: chars.map((c) => Math.max(1, beats(c.text))),
      onsetsMs: chars.map((c) => c.begin),
    })),
  });
  if (songs.length >= most) {
    break;
  }
}

const data = {
  source: `https://github.com/amll-dev/amll-ttml-db/tree/${commit}/raw-lyrics`,
  licence: "CC0-1.0",
  songs,
};
writeFileSync(out, JSON.stringify(data) + "\n");
console.log(`real-timings: wrote ${songs.length} songs to ${out.pathname}`);
