<script lang="ts">
  import {
    WordKind,
    type Line,
    type RubyPart,
    type Word,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

  // A line as word units: each word's Japanese, with its furigana, sits directly over its English, so it's always
  // clear which English belongs to which word. With onword set, each word is a button that opens the word sheet. With
  // placeholder set, the text is drawn as skeleton bars, for a line that stands in while a song loads. sweep is how far
  // through each piece the singing is, from 0 to 1, while the line is sung along with Spotify.
  let {
    line,
    furigana,
    onword,
    placeholder = false,
    sweep = null,
  }: {
    line: Line;
    furigana: boolean;
    onword?: (word: Word, ruby: RubyPart[]) => void;
    placeholder?: boolean;
    sweep?: number[] | null;
  } = $props();

  function label(word: Word): string {
    return word.short && word.short !== word.text ? `${word.text}, ${word.short}` : word.text;
  }
</script>

<!-- prettier-ignore -->
{#snippet japanese(parts: RubyPart[], sung: number | undefined)}<span class="ja" class:sweep={sung !== undefined} style:--sung={sung === undefined ? undefined : `${sung * 100}%`}><span class:ph={placeholder}>{#each parts as part, k (k)}{#if furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}</span></span>{/snippet}

<span class="il" class:furigana>
  {#each line.pieces ?? [] as piece, j (j)}
    {@const word = piece.word >= 0 ? line.words?.[piece.word] : undefined}
    {@const grammar = word !== undefined && word.kind !== WordKind.WordContent}
    {#if word && onword}
      <button
        type="button"
        class="unit word"
        class:grammar
        aria-label={label(word)}
        onclick={() => onword(word, piece.ruby ?? [])}
      >
        {@render japanese(piece.ruby ?? [], sweep?.[j])}
        <span class="en" lang="en" aria-hidden="true"><span class:ph={placeholder}>{word.short}</span></span>
      </button>
    {:else}
      <span class="unit" class:grammar class:bare={word === undefined}>
        {@render japanese(piece.ruby ?? [], sweep?.[j])}
        <span class="en" lang="en"><span class:ph={placeholder && word?.short}>{word?.short ?? ""}</span></span>
      </span>
    {/if}
  {/each}
</span>

<style>
  /* Units wrap like words in a sentence, centred, with room between them so each pair reads as one. They line up on
     the baseline of their Japanese, so a word with furigana sits level with one without: Safari grows a line for its
     ruby however much line height there is, which pushed those words, and the English under them, down. */
  .il {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    align-items: baseline;
    gap: 6px 4px;
  }

  .unit {
    display: inline-flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: 2px 6px 4px;
    border-radius: 10px;
  }

  /* Punctuation and other text that isn't a word sits close to its neighbours. */
  .bare {
    flex-shrink: 1;
    padding-inline: 0;
  }

  /* An English phrase in the line is one piece, so it wraps like text instead of running off the screen. */
  .bare .ja {
    white-space: normal;
  }

  .ja {
    white-space: nowrap;
    line-height: 1.3;
  }

  .furigana .ja {
    line-height: 2.1;
  }

  .ja :global(rt) {
    font-size: max(0.5em, 10px);
    font-weight: 400;
    line-height: 1;
    color: var(--muted);
  }

  /* The English is set apart from the Japanese by size, weight, and face, and a particle's English (its role, such as
     "(subject)") by colour, so content words and grammar don't blur together. */
  .en {
    max-width: 9em;
    min-height: 1.25em;
    font-family:
      system-ui,
      -apple-system,
      sans-serif;
    font-size: 0.875rem;
    font-weight: 500;
    line-height: 1.25;
    letter-spacing: 0;
    text-align: center;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .grammar .en {
    color: var(--link);
    font-style: italic;
  }

  /* In the current line each unit is a real button, at least 44px, with the Japanese underlined so it reads as
     tappable without hover. */
  .word {
    min-width: var(--tap);
    min-height: var(--tap);
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .word .ja {
    text-decoration: underline dotted var(--link);
    text-decoration-thickness: 2px;
    text-underline-offset: 6px;
  }

  .word:hover {
    background: var(--surface-strong);
  }
</style>
