<script lang="ts">
  import {
    Service as Songs,
    type RubyPart,
    type Song,
    type Word,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import DisplaySheet from "../components/DisplaySheet.svelte";
  import Interlinear from "../components/Interlinear.svelte";
  import WordSheet from "../components/WordSheet.svelte";
  import { toAppError, type AppError } from "../lib/errors";
  import { startsStanza } from "../lib/lyrics";
  import { meaningsCache } from "../lib/meaningsCache";
  import { hasRomaji, loadReadingAids, saveReadingAids, type ReadingAids } from "../lib/readingAids";
  import { aboutHref, homeHref } from "../lib/route";
  import { sourceLabel } from "../lib/sources";

  let { id }: { id: number } = $props();

  let song = $state<Song | null>(null);
  let failure = $state<AppError | null>(null);
  let current = $state(0);
  let deleting = $state(false);
  let aids = $state(loadReadingAids());
  let displayOpen = $state(false);
  let open = $state<{ word: Word; ruby: RubyPart[] } | null>(null);

  let lines = $derived(song?.lines ?? []);
  // JMdict's licence asks for a credit wherever its meanings show, so the footnote appears only when they do.
  let hasDefinitions = $derived(lines.some((l) => (l.words ?? []).some((w) => w.entryId > 0)));
  let source = $derived.by(() => {
    if (!song || song.source === "user") {
      return "";
    }
    return sourceLabel(song.source) + (lines.some((l) => l.startMs !== null) ? ", synced" : "");
  });

  function toggle(aid: keyof ReadingAids) {
    aids[aid] = !aids[aid];
    // When storage refuses, the choice still holds until the screen closes, which is all that can be done.
    saveReadingAids(aids);
  }

  $effect(() => {
    Songs.Get(id)
      .then((found) => {
        song = found;
        // Fetch every word's meanings now, in one call, so tapping a word opens its sheet at once.
        void meaningsCache.preload((found.lines ?? []).flatMap((line) => line.words ?? []));
      })
      .catch((err: unknown) => (failure = toAppError(err)));
  });

  async function remove() {
    deleting = true;
    try {
      await Songs.Delete(id);
      window.location.replace(homeHref);
    } catch (err) {
      failure = toAppError(err);
      deleting = false;
      displayOpen = false;
    }
  }
</script>

<!-- prettier-ignore -->
{#snippet rubyText(parts: RubyPart[])}{#each parts as part, k (k)}{#if aids.furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}{/snippet}

<div class="screen">
  <header class="topbar">
    <a class="icon-button" href={homeHref} aria-label="Back to your songs">
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"><polyline points="15 18 9 12 15 6" /></svg
      >
    </a>
    <div class="topbar-title">
      {#if song}
        <h1>{song.title}</h1>
        <p>{song.artist || "Unknown artist"}</p>
      {/if}
    </div>
    {#if song}
      <button
        type="button"
        class="icon-button display"
        aria-label="Display options"
        onclick={() => (displayOpen = true)}
      >
        <span aria-hidden="true">Aa</span>
      </button>
    {/if}
  </header>

  {#if failure?.code === "not_found"}
    <main class="center">
      <p class="empty-title">This song isn't in your library</p>
      <a class="button primary" href={homeHref}>Back to your songs</a>
    </main>
  {:else if failure}
    <main class="center">
      <p class="alert" role="alert">{failure.message}</p>
    </main>
  {:else if song === null}
    <main class="center"><p class="muted">Loading…</p></main>
  {:else}
    <main>
      <ol class="lyrics" class:furigana={aids.furigana} class:words={aids.words} lang="ja">
        {#each lines as line, i (line.position)}
          <li class:stanza-start={startsStanza(lines, i)}>
            {#if aids.words && (line.words ?? []).length > 0}
              {#if i === current}
                <div class="line current card" role="group" aria-label={line.text} aria-current="true">
                  <Interlinear {line} furigana={aids.furigana} onword={(word, ruby) => (open = { word, ruby })} />
                  {#if aids.romaji && hasRomaji(line)}
                    <span class="romaji" lang="ja-Latn">{line.romaji}</span>
                  {/if}
                </div>
              {:else}
                <button
                  type="button"
                  class="line card"
                  aria-current={false}
                  aria-label={line.text}
                  onclick={() => (current = i)}
                >
                  <Interlinear {line} furigana={aids.furigana} />
                  {#if aids.romaji && hasRomaji(line)}
                    <span class="romaji" lang="ja-Latn">{line.romaji}</span>
                  {/if}
                </button>
              {/if}
            {:else if i === current && (line.words ?? []).length > 0}
              <div class="line current" role="group" aria-label={line.text} aria-current="true">
                <!-- prettier-ignore -->
                <span class="sung">{#each line.pieces ?? [] as piece, j (j)}{@const word = piece.word >= 0 ? line.words?.[piece.word] : undefined}{#if word}<button type="button" class="word" aria-label={word.text} onclick={() => (open = { word, ruby: piece.ruby ?? [] })}>{@render rubyText(piece.ruby ?? [])}</button>{:else}{@render rubyText(piece.ruby ?? [])}{/if}{/each}</span>
                {#if aids.romaji && hasRomaji(line)}
                  <span class="romaji" lang="ja-Latn">{line.romaji}</span>
                {/if}
              </div>
            {:else}
              <button
                type="button"
                class="line"
                class:current={i === current}
                aria-current={i === current}
                aria-label={line.text}
                onclick={() => (current = i)}
              >
                <!-- prettier-ignore -->
                <span class="sung">{@render rubyText(line.ruby ?? [{ text: line.text, reading: "" }])}</span>
                {#if aids.romaji && hasRomaji(line)}
                  <span class="romaji" lang="ja-Latn">{line.romaji}</span>
                {/if}
              </button>
            {/if}
          </li>
        {/each}
      </ol>
      {#if aids.words && hasDefinitions}
        <p class="footnote">Definitions from JMdict · <a href={aboutHref}>Sources</a></p>
      {/if}
    </main>

    {#if open}
      <WordSheet word={open.word} ruby={open.ruby} onclose={() => (open = null)} />
    {/if}

    {#if displayOpen}
      <DisplaySheet
        {aids}
        {source}
        {deleting}
        ontoggle={toggle}
        ondelete={remove}
        onclose={() => (displayOpen = false)}
      />
    {/if}
  {/if}
</div>

<style>
  .display {
    font-size: 1.0625rem;
    font-weight: 700;
    letter-spacing: -0.02em;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 8px 0 max(32px, env(safe-area-inset-bottom));
  }

  .center {
    justify-content: center;
    align-items: center;
    gap: 16px;
    text-align: center;
  }

  .center .button {
    flex: none;
  }

  .lyrics {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .stanza-start {
    margin-top: 24px;
  }

  .line {
    display: block;
    width: 100%;
    min-height: var(--tap);
    padding: 6px 8px;
    border: 0;
    border-radius: 16px;
    background: transparent;
    color: var(--dim);
    font-size: 1.125rem;
    font-weight: 500;
    line-height: 1.7;
    text-align: center;
    cursor: pointer;
    transition:
      color 0.15s,
      font-size 0.15s;
  }

  .line:hover {
    color: var(--muted);
  }

  .line.current {
    color: var(--fg);
    font-size: 1.5rem;
    font-weight: 700;
  }

  .sung {
    display: block;
  }

  /* Furigana sits above the line inside its line height, so the rows need room for it. */
  .furigana .line {
    line-height: 2.2;
  }

  .line :global(rt) {
    font-size: max(0.5em, 10px);
    font-weight: 400;
    line-height: 1;
    color: inherit;
  }

  .line.current :global(rt) {
    color: var(--muted);
  }

  .romaji {
    display: block;
    margin-bottom: 4px;
    color: var(--dim);
    font-size: 0.9375rem;
    font-weight: 400;
    line-height: 1.4;
  }

  .line.current .romaji {
    color: var(--muted);
  }

  /* With Words on, each line is a soft card, so it's clear where one line and its English end and the next begin.
     The current line's card is brighter. */
  .words .card {
    margin: 4px 0;
    padding: 10px 6px;
    background: color-mix(in srgb, var(--surface) 55%, transparent);
  }

  .words .card.current {
    background: var(--surface);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--link) 45%, transparent);
  }

  .words .card .romaji {
    margin-top: 6px;
  }

  /* The words of the current line. Each is a real button, at least 44px each way, marked with an amber underline so
     it reads as tappable without hover. */
  .word {
    display: inline-block;
    min-width: var(--tap);
    min-height: var(--tap);
    margin: 0 1px;
    padding: 0 2px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    font: inherit;
    line-height: inherit;
    text-decoration: underline dotted var(--link);
    text-decoration-thickness: 2px;
    text-underline-offset: 6px;
    cursor: pointer;
  }

  .word:hover {
    background: var(--surface);
  }

  @media (prefers-reduced-motion: reduce) {
    .line {
      transition: none;
    }
  }

  @media (min-width: 640px) {
    .line {
      font-size: 1.25rem;
    }

    .line.current {
      font-size: 1.75rem;
    }
  }
</style>
