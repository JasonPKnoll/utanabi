import {
  WordKind,
  type Line,
  type RubyPart,
  type Word,
} from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
import type { Card } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/study";

// Stand-in content for skeletons. A screen renders it through its own markup while the real content loads, with its
// text drawn as shimmering bars (the .ph class), so the skeleton takes the real layout's shape: the same fonts, line
// heights, furigana, romaji, and word cards. Only the amounts of text are made up, and they're fixed, so a skeleton
// looks the same on every load.

// A stand-in word: how many characters it has, and whether it's a content word, which gets furigana, a reading, and
// a longer English gloss under it, or a particle.
type Shape = [chars: number, content: boolean];

// Eight lines in two verses, of mixed lengths, like a song's.
const lineShapes: Shape[][] = [
  [
    [2, true],
    [1, false],
    [2, true],
    [1, false],
    [3, true],
  ],
  [
    [3, true],
    [1, false],
    [4, true],
  ],
  [
    [2, true],
    [1, false],
    [3, true],
    [2, true],
  ],
  [
    [1, true],
    [1, false],
    [2, true],
    [1, false],
    [2, true],
    [1, false],
  ],
  [
    [4, true],
    [1, false],
    [2, true],
  ],
  [
    [2, true],
    [2, true],
    [1, false],
    [3, true],
    [1, false],
  ],
  [
    [3, true],
    [1, false],
    [2, true],
  ],
  [
    [2, true],
    [1, false],
    [2, true],
    [1, false],
    [4, true],
  ],
];

function textOf(chars: number, content: boolean): string {
  return (content ? "字" : "の").repeat(chars);
}

function word(index: number, chars: number, content: boolean): Word {
  return {
    firstToken: index,
    lastToken: index,
    text: textOf(chars, content),
    lemma: textOf(chars, content),
    lemmaReading: "",
    lemmaRomaji: "",
    kind: content ? WordKind.WordContent : WordKind.WordParticle,
    conjClass: "",
    entryId: 0,
    senseIndex: null,
    gloss: "",
    short: content ? "meaning".slice(0, 3 + chars) : "(of)",
    grammar: null,
    steps: [],
    form: "",
    sung: "",
    sungRomaji: "",
  };
}

// Furigana over every other content word, one kana a character: lyrics mix kanji and kana, and furigana only sits
// over kanji.
function rubyOf(index: number, chars: number, content: boolean): RubyPart[] {
  return [{ text: textOf(chars, content), reading: content && index % 4 === 0 ? "か".repeat(chars) : "" }];
}

// A song's lines while it loads.
export function placeholderLines(): Line[] {
  return lineShapes.map((shapes, position) => {
    const words = shapes.map(([chars, content], i) => word(i, chars, content));
    const pieces = shapes.map(([chars, content], i) => ({ ruby: rubyOf(i, chars, content), word: i }));
    return {
      position,
      stanza: position < 4 ? 0 : 1,
      text: words.map((w) => w.text).join(""),
      startMs: null,
      ruby: pieces.flatMap((p) => p.ruby),
      romaji: shapes.map(([chars]) => "ro".repeat(chars)).join(" "),
      words,
      pieces,
      gloss: "",
    };
  });
}

// The library's songs while they load: a title and an artist line each, of mixed lengths.
export const placeholderSongs = [
  { title: "A song title", artist: "Artist name" },
  { title: "A longer song title here", artist: "An artist" },
  { title: "Song title", artist: "Artist name here" },
  { title: "Another song title", artist: "Artist" },
  { title: "A song", artist: "An artist name" },
];

// A deck's word cards while they load.
export function placeholderCards(): Card[] {
  const shapes: [label: string, meaning: string][] = [
    ["字字", "a meaning; another"],
    ["字字字", "a meaning"],
    ["字字", "a longer meaning; and more"],
    ["字字字字", "meaning"],
    ["字", "a meaning; another one"],
    ["字字字", "a meaning here"],
  ];
  return shapes.map(([label, meaning], i) => ({
    id: -(i + 1),
    label,
    reading: "かな".repeat(label.length),
    readings: [],
    askReading: true,
    meaning,
    meanings: [],
    line: "",
    wordStart: 0,
    wordText: "",
    progress: { streak: 0, right: 0, wrong: 0, learned: false },
  }));
}
