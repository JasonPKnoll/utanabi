<script lang="ts">
  import { Service as Songs, type Song } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
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

  let lines = $derived(song?.lines ?? []);

  function toggle(aid: keyof ReadingAids) {
    aids[aid] = !aids[aid];
    // When storage refuses, the choice still holds until the screen closes, which is all that can be done.
    saveReadingAids(aids);
  }

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
      </div>
      <p class="hint muted">
        Tap a line to follow along.
        {#if song.source !== "user"}
          <br />Lyrics from {sourceLabel(song.source)}{song.lines?.some((l) => l.startMs !== null) ? ", synced" : ""}.
        {/if}
      </p>
      <ol class="lyrics" class:furigana={aids.furigana} lang="ja">
        {#each lines as line, i (line.position)}
          <li class:stanza-start={startsStanza(lines, i)}>
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
          </li>
        {/each}
      </ol>
    </main>

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
    justify-content: center;
    gap: 12px;
    margin: 4px 0 8px;
  }

  .pill {
    min-height: var(--tap);
    min-width: 96px;
    padding: 0 18px;
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
