<script lang="ts">
  import { tick } from "svelte";

  import {
    Service as Study,
    type Card,
    type Deck,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/study";
  import { toAppError, type AppError } from "../lib/errors";
  import { quizHref, songHref, studyHref } from "../lib/route";
  import { lineParts, otherMeanings, placeOf, shownCards, startFilter, type Filter } from "../lib/studyList";

  // A song's words to study before the quiz: a list of every card with its reading and meaning, and each card on its
  // own, stepped through with Previous and Next (or a swipe, or the arrow keys). Nothing here is an answer: only the
  // quiz changes a card's progress.
  let { songId, cardId }: { songId: number; cardId?: number } = $props();

  let deck = $state<Deck | null>(null);
  let failure = $state<AppError | null>(null);
  let filter = $state<Filter>("learn");
  // The last card opened, so the list comes back to it.
  let lastCard: number | undefined;

  let cards = $derived(deck?.cards ?? []);
  let learned = $derived(cards.filter((c) => c.progress.learned).length);
  let goal = $derived(deck?.goal ?? 3);
  let list = $derived(shownCards(cards, filter));
  let card = $derived(cardId === undefined ? undefined : cards.find((c) => c.id === cardId));
  // A card opened from outside the list shown, such as a learned one by its link, steps through every card.
  let steps = $derived(card && placeOf(list, card.id).index < 0 ? cards : list);
  let place = $derived(card ? placeOf(steps, card.id) : { index: -1 });

  $effect(() => {
    Study.Deck(songId)
      .then((d) => {
        filter = startFilter(d.cards ?? []);
        deck = d;
      })
      .catch((err: unknown) => (failure = toAppError(err)));
  });

  $effect(() => {
    if (cardId !== undefined) {
      lastCard = cardId;
      return;
    }
    if (deck && lastCard !== undefined) {
      const id = lastCard;
      void tick().then(() => {
        const row = document.getElementById(`card-${id}`);
        row?.scrollIntoView({ block: "center" });
        row?.focus({ preventScroll: true });
      });
    }
  });

  // Stepping replaces the card in the history, so Back goes to the list, not through every card seen.
  function step(id: number | undefined) {
    if (id !== undefined) {
      window.location.replace(studyHref(songId, id));
    }
  }

  function onkeydown(event: KeyboardEvent) {
    if (!card || event.altKey || event.ctrlKey || event.metaKey) {
      return;
    }
    if (event.key === "ArrowLeft") {
      step(place.previous);
    } else if (event.key === "ArrowRight") {
      step(place.next);
    }
  }

  let touch: { x: number; y: number } | null = null;

  function ontouchstart(event: TouchEvent) {
    const t = event.touches[0];
    touch = event.touches.length === 1 && t ? { x: t.clientX, y: t.clientY } : null;
  }

  function ontouchend(event: TouchEvent) {
    const t = event.changedTouches[0];
    if (!touch || !t) {
      return;
    }
    const dx = t.clientX - touch.x;
    const dy = t.clientY - touch.y;
    touch = null;
    // A sideways swipe, not a scroll or a tap.
    if (Math.abs(dx) > 60 && Math.abs(dx) > 2 * Math.abs(dy)) {
      step(dx < 0 ? place.next : place.previous);
    }
  }

  function streakText(c: Card): string {
    if (c.progress.learned) {
      return "Learned";
    }
    if (c.progress.right + c.progress.wrong === 0) {
      return "Not quizzed yet";
    }
    return `${c.progress.streak} of ${goal} in a row`;
  }
</script>

<svelte:window {onkeydown} />

{#snippet back(href: string, label: string)}
  <a class="icon-button" {href} aria-label={label}>
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
{/snippet}

{#snippet dots(c: Card, size: string)}
  <span class="dots" style:--dot={size} aria-hidden="true">
    {#each Array.from({ length: goal }, (_, i) => i) as i (i)}
      <span class:on={c.progress.learned || i < c.progress.streak}></span>
    {/each}
  </span>
{/snippet}

<div class="screen">
  {#if cardId !== undefined}
    <header class="topbar">
      {@render back(studyHref(songId), "Back to the word list")}
      <div class="topbar-title">
        <h1>{place.index >= 0 ? `Word ${place.index + 1} of ${steps.length}` : "Word"}</h1>
        {#if deck}<p>{deck.title}</p>{/if}
      </div>
    </header>
  {:else}
    <header class="topbar">
      {@render back(songHref(songId), "Back to the song")}
      <div class="topbar-title">
        <h1>Study</h1>
        {#if deck}<p>{deck.title}</p>{/if}
      </div>
      {#if cards.length > 0}
        <p class="count" aria-label="{learned} of {cards.length} words learned">{learned}/{cards.length}</p>
      {/if}
    </header>
    {#if cards.length > 0}
      <div class="bar" aria-hidden="true"><span style:width="{(learned / cards.length) * 100}%"></span></div>
    {/if}
  {/if}

  {#if failure?.code === "not_found"}
    <main class="center"><p class="empty-title">This song isn't in your library</p></main>
  {:else if failure}
    <main class="center"><p class="alert" role="alert">{failure.message}</p></main>
  {:else if deck === null}
    <main>
      <p class="visually-hidden" role="status">Loading the words</p>
      <div class="skeleton-wait" aria-hidden="true">
        <span class="skeleton filter-skeleton"></span>
        <ul class="words">
          {#each [56, 40, 64, 48, 60, 44] as width (width)}
            <li>
              <div class="word-row">
                <span class="ja">
                  <span class="skeleton" style:width="{width}%" style:height="20px"></span>
                  <span class="skeleton" style:width="{width - 14}%" style:height="10px" style:margin-top="6px"></span>
                </span>
                <span class="skeleton" style:width="{110 - width}%" style:height="14px"></span>
                <span class="skeleton" style:width="26px" style:height="7px"></span>
              </div>
            </li>
          {/each}
        </ul>
      </div>
    </main>
  {:else if cards.length === 0}
    <main class="center">
      <div class="empty">
        <p class="empty-title">No words to study yet</p>
        <p class="muted">
          {deck.installed
            ? "None of this song's words are in the dictionary."
            : "Studying needs the dictionary of word meanings, which isn't installed."}
        </p>
      </div>
      <a class="button" href={songHref(songId)}>Back to the song</a>
    </main>
  {:else if cardId !== undefined && !card}
    <main class="center">
      <p class="empty-title">This word isn't in this song</p>
      <a class="button" href={studyHref(songId)}>Back to the word list</a>
    </main>
  {:else if card}
    {@const [before, word, after] = lineParts(card)}
    {@const others = otherMeanings(card.meaning, card.meanings ?? [])}
    <main class="lesson" {ontouchstart} {ontouchend}>
      <section class="card" aria-live="polite">
        {#if card.askReading}
          <p class="reading" lang="ja">{card.reading}</p>
        {/if}
        <p class="word" lang="ja">{card.label}</p>
        <p class="meaning">{card.meaning || "No meaning in the dictionary"}</p>
        {#if others.length > 0}
          <p class="others"><span>Also</span> {others.slice(0, 6).join(", ")}</p>
        {/if}
        {#if card.line}
          <p class="line" lang="ja">{before}<mark>{word}</mark>{after}</p>
        {/if}
        <div class="progress">
          {@render dots(card, "10px")}
          <p class:learned={card.progress.learned}>{streakText(card)}</p>
        </div>
      </section>
    </main>
    <div class="actions">
      <button type="button" class="button" disabled={place.previous === undefined} onclick={() => step(place.previous)}>
        Previous
      </button>
      {#if place.next !== undefined}
        <button type="button" class="button primary" onclick={() => step(place.next)}>Next</button>
      {:else}
        <a class="button primary" href={quizHref(songId)}>Quiz me</a>
      {/if}
    </div>
  {:else}
    <main>
      <fieldset class="filter">
        <legend class="visually-hidden">Show</legend>
        <label class:chosen={filter === "learn"}>
          <input type="radio" name="filter" value="learn" bind:group={filter} />
          To learn <span class="n">{cards.length - learned}</span>
        </label>
        <label class:chosen={filter === "all"}>
          <input type="radio" name="filter" value="all" bind:group={filter} />
          All <span class="n">{cards.length}</span>
        </label>
      </fieldset>

      {#if list.length === 0}
        <div class="empty">
          <p class="empty-title">Every word is learned</p>
          <p class="muted">Each one was right {goal} times in a row. Choose All to go over them again.</p>
        </div>
      {:else}
        <ul class="words">
          {#each list as c (c.id)}
            <li>
              <a id="card-{c.id}" class="word-row" href={studyHref(songId, c.id)}>
                <span class="ja">
                  <span class="label" lang="ja">{c.label}</span>
                  {#if c.askReading}<span class="kana" lang="ja">{c.reading}</span>{/if}
                </span>
                <span class="gloss">{c.meaning}</span>
                {@render dots(c, "7px")}
                <span class="visually-hidden">, {streakText(c)}</span>
              </a>
            </li>
          {/each}
        </ul>
      {/if}
    </main>
    <div class="actions">
      {#if list[0]}
        <a class="button" href={studyHref(songId, list[0].id)}>Flip through</a>
      {/if}
      <a class="button primary" href={quizHref(songId)}>Quiz me</a>
    </div>
  {/if}
</div>

<style>
  .count {
    margin: 0;
    padding: 0 8px;
    font-size: 0.875rem;
    font-weight: 650;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .bar {
    height: 4px;
    margin: 0 0 12px;
    border-radius: 2px;
    background: var(--surface-strong);
    overflow: hidden;
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--success);
  }

  main {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .center {
    flex: 1;
    justify-content: center;
    gap: 12px;
    text-align: center;
    padding-bottom: max(24px, env(safe-area-inset-bottom));
  }

  .center .button {
    flex: none;
  }

  .filter-skeleton {
    height: 52px;
    margin-bottom: 12px;
    border-radius: 14px;
  }

  .filter {
    display: flex;
    gap: 4px;
    margin: 0;
    padding: 4px;
    border: 0;
    border-radius: 14px;
    background: var(--surface);
  }

  .filter label {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: var(--tap);
    border-radius: 10px;
    color: var(--muted);
    font-weight: 600;
    cursor: pointer;
  }

  .filter label.chosen {
    background: var(--surface-strong);
    color: var(--fg);
  }

  .filter label:has(:focus-visible) {
    outline: 3px solid var(--link);
    outline-offset: 2px;
  }

  .filter input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .n {
    font-size: 0.8125rem;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
  }

  .words {
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--surface);
    overflow: hidden;
  }

  .words li + li {
    border-top: 1px solid var(--surface-strong);
  }

  .word-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr) auto;
    align-items: center;
    gap: 12px;
    min-height: 60px;
    padding: 8px 14px 8px 16px;
    color: var(--fg);
    text-decoration: none;
  }

  .word-row:hover {
    color: var(--fg);
    background: var(--surface-strong);
  }

  .word-row:focus-visible {
    outline-offset: -3px;
  }

  .ja {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .label {
    font-size: 1.25rem;
    font-weight: 650;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .kana {
    font-size: 0.8125rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .gloss {
    font-size: 0.9375rem;
    color: var(--muted);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .dots {
    display: flex;
    gap: calc(var(--dot) * 0.6);
  }

  .dots span {
    width: var(--dot);
    height: var(--dot);
    border-radius: 50%;
    background: var(--surface-strong);
  }

  .dots span.on {
    background: var(--success);
  }

  .lesson {
    flex: 1;
    justify-content: center;
    padding: 8px 0;
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 32px 20px 24px;
    border-radius: 20px;
    background: var(--surface);
    text-align: center;
  }

  .card p {
    margin: 0;
  }

  .reading {
    font-size: 1.375rem;
    font-weight: 600;
    color: var(--link);
  }

  .word {
    font-size: 3rem;
    font-weight: 750;
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .meaning {
    margin-top: 8px !important;
    font-size: 1.25rem;
    font-weight: 600;
  }

  .others {
    color: var(--muted);
    font-size: 0.9375rem;
  }

  .others span {
    font-size: 0.75rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .line {
    margin-top: 16px !important;
    padding-top: 16px;
    border-top: 1px solid var(--surface-strong);
    width: 100%;
    color: var(--muted);
  }

  mark {
    background: transparent;
    color: var(--fg);
    font-weight: 700;
    text-decoration: underline dotted var(--link);
    text-decoration-thickness: 2px;
    text-underline-offset: 5px;
  }

  .progress {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    margin-top: 12px;
    font-size: 0.875rem;
    color: var(--muted);
  }

  .progress .learned {
    color: var(--success);
    font-weight: 650;
  }
</style>
