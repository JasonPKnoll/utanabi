<script lang="ts">
  import {
    Service as Songs,
    type RubyPart,
    type Song,
    type Word,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { AboutService } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import Interlinear from "../components/Interlinear.svelte";
  import WordSheet from "../components/WordSheet.svelte";
  import { toAppError, type AppError } from "../lib/errors";
  import { startsStanza } from "../lib/lyrics";
  import { hasRomaji, loadReadingAids, saveReadingAids, type ReadingAids } from "../lib/readingAids";
  import { homeHref } from "../lib/route";
  import { sourceLabel } from "../lib/sources";

  let { id }: { id: number } = $props();

  let song = $state<Song | null>(null);
  let failure = $state<AppError | null>(null);
  let current = $state(0);
  let confirmDelete = $state(false);
  let deleting = $state(false);
  let aids = $state(loadReadingAids());
  // JMdict's licence asks for its credit on every page that shows its words, so it sits under the glosses.
  let jmdictCredit = $state("");
  let open = $state<{ word: Word; ruby: RubyPart[] } | null>(null);

  let lines = $derived(song?.lines ?? []);

  function toggle(aid: keyof ReadingAids) {
    aids[aid] = !aids[aid];
    // When storage refuses, the choice still holds until the screen closes, which is all that can be done.
    saveReadingAids(aids);
  }

  $effect(() => {
    AboutService.Info()
      .then((about) => (jmdictCredit = about.credits?.find((c) => c.name === "JMdict")?.text ?? ""))
      .catch(() => (jmdictCredit = ""));
  });

  $effect(() => {
    Songs.Get(id)
      .then((found) => (song = found))
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
      confirmDelete = false;
    }
  }
</script>

<div class="screen">
  <header>
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
    {#if song}
      <div class="heading">
        <h1>{song.title}</h1>
        <p class="muted">{song.artist || "Unknown artist"}</p>
      </div>
    {/if}
    <span class="spacer" aria-hidden="true"></span>
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
      <div class="aids" role="group" aria-label="Reading aids">
        <button type="button" class="pill" aria-pressed={aids.furigana} onclick={() => toggle("furigana")}>
          Furigana
        </button>
        <button type="button" class="pill" aria-pressed={aids.romaji} onclick={() => toggle("romaji")}>Romaji</button>
        <button type="button" class="pill" aria-pressed={aids.words} onclick={() => toggle("words")}>Words</button>
      </div>
      <p class="hint muted">
        Tap a line to follow along, then tap a word in it to see what it means.
        {#if song.source !== "user"}
          <br />Lyrics from {sourceLabel(song.source)}{song.lines?.some((l) => l.startMs !== null) ? ", synced" : ""}.
        {/if}
      </p>
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
                <span class="sung">{#each line.pieces ?? [] as piece, j (j)}{@const word = piece.word >= 0 ? line.words?.[piece.word] : undefined}{#if word}<button type="button" class="word" aria-label={word.text} onclick={() => (open = { word, ruby: piece.ruby ?? [] })}>{#each piece.ruby ?? [] as part, k (k)}{#if aids.furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}</button>{:else}{#each piece.ruby ?? [] as part, k (k)}{#if aids.furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}{/if}{/each}</span>
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
                <span class="sung">{#each line.ruby ?? [{ text: line.text, reading: "" }] as part, j (j)}{#if aids.furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}</span>
                {#if aids.romaji && hasRomaji(line)}
                  <span class="romaji" lang="ja-Latn">{line.romaji}</span>
                {/if}
              </button>
            {/if}
          </li>
        {/each}
      </ol>
      {#if aids.words && jmdictCredit}
        <p class="credit">{jmdictCredit}</p>
      {/if}
    </main>

    {#if open}
      <WordSheet word={open.word} ruby={open.ruby} onclose={() => (open = null)} />
    {/if}

    <div class="actions">
      {#if confirmDelete}
        <button type="button" class="button" onclick={() => (confirmDelete = false)} disabled={deleting}>Keep it</button
        >
        <button type="button" class="button danger" onclick={remove} disabled={deleting}>
          {deleting ? "Deleting…" : "Delete song"}
        </button>
      {:else}
        <button type="button" class="button danger" onclick={() => (confirmDelete = true)}>Delete song…</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 0;
    margin: 0 -8px;
  }

  .heading {
    flex: 1;
    min-width: 0;
    text-align: center;
  }

  .heading h1,
  .heading p {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .heading h1 {
    font-size: 1.1rem;
    font-weight: 700;
  }

  .heading p {
    font-size: 0.85rem;
  }

  .spacer {
    flex: none;
    width: var(--tap);
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding-bottom: 16px;
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

  .aids {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
    margin: 4px 0 8px;
  }

  .pill {
    min-height: var(--tap);
    min-width: 88px;
    padding: 0 16px;
    border: 1px solid var(--border);
    border-radius: 22px;
    background: var(--surface);
    color: var(--muted);
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
  }

  .pill[aria-pressed="true"] {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--on-accent);
  }

  .credit {
    margin: 24px 0 0;
    font-size: 0.8rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .hint {
    margin: 4px 0 12px;
    font-size: 0.85rem;
    text-align: center;
  }

  .lyrics {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .stanza-start {
    margin-top: 20px;
  }

  .line {
    display: block;
    width: 100%;
    min-height: var(--tap);
    padding: 6px 8px;
    border: 0;
    border-radius: 12px;
    background: transparent;
    color: var(--dim);
    font-size: 1.15rem;
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
    font-size: 1.6rem;
    font-weight: 700;
  }

  .sung {
    display: block;
  }

  /* Furigana sits above the line inside its line height, so the rows need room for it. */
  .furigana .line {
    line-height: 2.2;
  }

  .line rt {
    font-size: max(0.5em, 10px);
    font-weight: 400;
    line-height: 1;
    color: inherit;
  }

  .line.current rt {
    color: var(--muted);
  }

  .romaji {
    display: block;
    margin-bottom: 4px;
    color: var(--dim);
    font-size: 1rem;
    font-weight: 400;
    line-height: 1.4;
  }

  .line.current .romaji {
    color: var(--muted);
  }

  /* With Words on, each line is its own card, so it's clear where one line and its English end and the next begin.
     The current line's card is outlined in amber. */
  .words .card {
    margin: 6px 0;
    padding: 10px 6px;
    border: 1px solid var(--border);
    background: var(--surface);
  }

  .words .card.current {
    border-color: var(--link);
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
      font-size: 1.3rem;
    }

    .line.current {
      font-size: 1.9rem;
    }
  }
</style>
