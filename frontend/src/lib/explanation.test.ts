import { describe, expect, it } from "vitest";

import grammar from "../../../internal/dictionary/grammar.json";

import { structure } from "./explanation";

describe("structure", () => {
  it("sets the examples after a colon under their rule", () => {
    const parts = structure(
      "でも after a noun stretches it: 子供でもわかる, even a child understands. After a question word it means " +
        '"any": いつでも, any time; 何度でも, as many times as it takes. It can also offer something as an example: ' +
        "お茶でも飲もう, let's have tea or something.",
    );
    expect(parts).toEqual([
      { text: "でも after a noun stretches it:", examples: [{ ja: "子供でもわかる", en: "even a child understands" }] },
      {
        text: 'After a question word it means "any":',
        examples: [
          { ja: "いつでも", en: "any time" },
          { ja: "何度でも", en: "as many times as it takes" },
        ],
      },
      {
        text: "It can also offer something as an example:",
        examples: [{ ja: "お茶でも飲もう", en: "let's have tea or something" }],
      },
    ]);
  });

  it("takes bracketed examples out of their sentence", () => {
    const parts = structure(
      "The te form joins a verb or adjective to what follows. It can list actions in order (起きて、歌った, got up " +
        "and sang), or make a request on its own (聞いて, listen).",
    );
    expect(parts).toEqual([
      { text: "The te form joins a verb or adjective to what follows.", examples: [] },
      {
        text: "It can list actions in order, or make a request on its own.",
        examples: [
          { ja: "起きて、歌った", en: "got up and sang" },
          { ja: "聞いて", en: "listen" },
        ],
      },
    ]);
  });

  it("leaves prose that only mentions Japanese as prose, in one paragraph", () => {
    const text =
      "Compare は, which marks what the sentence is about. The subject of a smaller clause takes が, not は.";
    expect(structure(text)).toEqual([{ text, examples: [] }]);
  });

  it("keeps a question mark inside a sentence, and a quoted word after a colon, as prose", () => {
    const parts = structure(
      "With a question word it wonders: どこにいるんだろう, where could you be? With rising intonation it asks " +
        'for agreement: "right?". As a question, 〜の? or 〜んですか asks for the story.',
    );
    expect(parts).toEqual([
      { text: "With a question word it wonders:", examples: [{ ja: "どこにいるんだろう", en: "where could you be?" }] },
      {
        text: 'With rising intonation it asks for agreement: "right?". As a question, 〜の? or 〜んですか asks for the story.',
        examples: [],
      },
    ]);
  });
});

describe("structure on the app's grammar list", () => {
  const points = grammar as { id: string; explanation: string }[];
  // Only letters and kana count, so moving punctuation around isn't a loss.
  const letters = (s: string) => s.replace(/[\s.,:;()"“”]/gu, "");

  it("loses none of any explanation's text", () => {
    for (const p of points) {
      const back = structure(p.explanation)
        .map((part) => part.text + part.examples.map((e) => e.ja + e.en).join(""))
        .join("");
      expect([...letters(back)].sort().join(""), p.id).toBe([...letters(p.explanation)].sort().join(""));
    }
  });

  it("finds examples in most explanations", () => {
    const withExamples = points.filter((p) => structure(p.explanation).some((part) => part.examples.length > 0));
    expect(withExamples.length).toBeGreaterThan(points.length * 0.8);
  });
});
