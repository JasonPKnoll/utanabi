import { toHiragana, toKatakana } from "wanakana";

// How a word is sung when that isn't how it's written (gikun): 羅針盤 written "compass" and sung コンパス. The reading is
// typed in kana, or in romaji that turns into kana as it's typed, in the script chosen.

export type Script = "katakana" | "hiragana";

// The field's text as it's typed: romaji becomes kana in the chosen script, and a lone n waits for the next letter.
export function typeSung(raw: string, script: Script): string {
  return script === "katakana" ? toKatakana(raw, { IMEMode: true }) : toHiragana(raw, { IMEMode: true });
}

// The reading to save: spaces dropped, and what's left, a final n too, turned into the chosen script.
export function finishSung(raw: string, script: Script): string {
  const text = raw.replace(/\s+/g, "");
  return script === "katakana" ? toKatakana(text) : toHiragana(text);
}

// The script a word's editor opens in: the one its reading is written in, or katakana, which most sung readings that
// differ from the writing are (loanwords such as コンパス).
export function scriptOf(reading: string): Script {
  return reading !== "" && reading === toHiragana(reading) ? "hiragana" : "katakana";
}
