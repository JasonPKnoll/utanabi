// A grammar point's explanation, cut into short parts so it reads in steps instead of as one block: each rule on its
// own, with its examples set apart under it. The explanations in internal/dictionary/grammar.json write examples
// inline, Japanese then English, in one of two ways, and this reads both:
//
//   After a question word it means "any": いつでも, any time; 何度でも, as many times as it takes.
//   It can list actions in order (起きて、歌った, got up and sang), or make a request (聞いて, listen).
//
// A sentence becomes a rule with examples only when every piece fits, so prose that merely mentions Japanese
// ("Compare は, which marks the topic") is left as it is.

export interface Example {
  ja: string;
  en: string;
}

export interface ExplanationPart {
  text: string;
  examples: Example[];
}

// Japanese as examples write it: kana, kanji, and the marks lyrics use, with digits for counters.
const ja = "[\\u3040-\\u30ff\\u3400-\\u9fff\\uff00-\\uffef々〜ー・、。！？「」…0-9]+";
const exampleItem = new RegExp(`^(${ja}), (.+)$`, "u");
const bracketed = new RegExp(`\\s?\\((${ja}), ([^()]+)\\)`, "gu");

// Sentences end at . ? or ! (after a closing quote or bracket too) before one that starts with a capital, a quote,
// or Japanese. So "〜の? or 〜んですか" stays one sentence.
const sentenceBreak = /(?<=[.?!]["”)]?)\s+(?=[A-Z"“「぀-ヿ㐀-鿿〜])/u;

export function structure(explanation: string): ExplanationPart[] {
  const parts: ExplanationPart[] = [];
  for (const sentence of explanation.trim().split(sentenceBreak)) {
    const part = afterColon(sentence) ?? inBrackets(sentence) ?? { text: sentence, examples: [] };
    const last = parts.at(-1);
    // Sentences with no examples run on together as one paragraph.
    if (part.examples.length === 0 && last && last.examples.length === 0) {
      last.text += " " + part.text;
    } else {
      parts.push(part);
    }
  }
  return parts;
}

// "Rule: 例, example; 例, example." The colon stays on the rule, so it still leads into the examples.
function afterColon(sentence: string): ExplanationPart | null {
  for (let at = sentence.indexOf(": "); at >= 0; at = sentence.indexOf(": ", at + 1)) {
    const items = sentence
      .slice(at + 2)
      .replace(/\.$/, "")
      .split("; ");
    const examples = items.map((item) => exampleItem.exec(item)).map((m) => (m ? { ja: m[1], en: m[2] } : null));
    if (examples.every((e) => e !== null)) {
      return { text: sentence.slice(0, at + 1), examples: examples as Example[] };
    }
  }
  return null;
}

// "A rule (例, example), and another (例, example)." The examples come out of the sentence and go under it.
function inBrackets(sentence: string): ExplanationPart | null {
  const examples = [...sentence.matchAll(bracketed)].map((m) => ({ ja: m[1] ?? "", en: m[2] ?? "" }));
  if (examples.length === 0) {
    return null;
  }
  return { text: sentence.replace(bracketed, ""), examples };
}
