<script lang="ts">
  import {
    Service as Songs,
    Tools,
    type Summary,
    type TimedSong,
    type ToolResult,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { settingsHref, songHref } from "../lib/route";

  // For admins, for testing: find songs the word timing database has and add them, with their word times, and add
  // any of your songs again fresh, as if for the first time. On only on a server with UTANABI_SONG_TOOLS on.
  let query = $state("");
  let timed = $state<TimedSong[] | null>(null);
  let mine = $state<Summary[] | null>(null);
  let failure = $state<AppError | null>(null);
  // The sheet or song a tool is working on, and what each one did.
  let working = $state("");
  let results = $state<Record<string, ToolResult | string>>({});
  // The song you asked to add again, until you say yes: it's removed from every library first.
  let confirming = $state<number | null>(null);

  async function find() {
    failure = null;
    try {
      timed = (await Tools.TimedSongs(query)) ?? [];
    } catch (err) {
      failure = toAppError(err);
    }
  }

  async function loadMine() {
    try {
      mine = (await Songs.List()) ?? [];
    } catch (err) {
      failure = toAppError(err);
    }
  }
  void find();
  void loadMine();

  async function run(key: string, fn: () => Promise<ToolResult>) {
    working = key;
    try {
      const result = await fn();
      // A song added again gets a new ID: what pointed at the old one is gone with it.
      const gone = key.startsWith("song-") ? Number(key.slice(5)) : 0;
      const kept = Object.entries(results).filter(([, r]) => typeof r === "string" || r.songId !== gone);
      results = { ...Object.fromEntries(kept), [key]: result };
      await loadMine();
    } catch (err) {
      results = { ...results, [key]: toAppError(err).message };
    } finally {
      working = "";
    }
  }

  function told(key: string): string {
    const r = results[key];
    if (r === undefined) {
      return "";
    }
    if (typeof r === "string") {
      return r;
    }
    return r.timed > 0
      ? `Added: ${r.timed} of ${r.lines} lines timed${r.source ? ` from ${r.source}` : ""}.`
      : `Added: ${r.lines} lines, none timed.`;
  }

  function added(key: string): ToolResult | null {
    const r = results[key];
    return r !== undefined && typeof r !== "string" ? r : null;
  }
</script>

<div class="screen">
  <header class="topbar">
    <a class="icon-button" href={settingsHref} aria-label="Back to settings">
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
    <div class="topbar-title"><h1>Song tools</h1></div>
  </header>

  <main>
    {#if failure?.code === "forbidden" || failure?.code === "not_found"}
      <p class="alert" role="alert">The song tools are for admins, on a server with them turned on.</p>
    {:else}
      {#if failure}
        <p class="alert" role="alert">{failure.message}</p>
      {/if}

      <section>
        <h2>Songs with word times</h2>
        <p class="muted intro">
          Songs the AMLL TTML DB has timed word by word. Adding one finds LRCLIB's lyrics that its times fit.
        </p>
        <form
          class="search"
          role="search"
          onsubmit={(event) => {
            event.preventDefault();
            void find();
          }}
        >
          <label for="tools-query" class="visually-hidden">Title or artist</label>
          <input
            id="tools-query"
            type="search"
            bind:value={query}
            placeholder="Title or artist"
            enterkeyhint="search"
            autocomplete="off"
          />
          <button type="submit" class="button">Find</button>
        </form>
        {#if timed === null}
          <p class="muted">Loading…</p>
        {:else if timed.length === 0}
          <p class="muted">No song there matches.</p>
        {:else}
          <ul class="list">
            {#each timed as song (song.sheet)}
              {@const done = added(song.sheet)}
              <li>
                <span class="text">
                  <span class="title">{song.title}</span>
                  <span class="sub">{song.artist || "Unknown artist"}</span>
                  {#if told(song.sheet)}<span class="result" role="status">{told(song.sheet)}</span>{/if}
                </span>
                {#if done}
                  <a class="button" href={songHref(done.songId)}>Open</a>
                {:else}
                  <button
                    type="button"
                    class="button primary"
                    disabled={working !== ""}
                    onclick={() => run(song.sheet, () => Tools.AddTimedSong(song.sheet))}
                    >{working === song.sheet ? "Adding…" : "Add"}</button
                  >
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <section>
        <h2>Your songs</h2>
        <p class="muted intro">
          Adding one again removes it from every library on this server, with its timings and cards, then adds it fresh
          from LRCLIB and looks its word times up.
        </p>
        {#if mine === null}
          <p class="muted">Loading…</p>
        {:else if mine.length === 0}
          <p class="muted">No songs yet.</p>
        {:else}
          <ul class="list">
            {#each mine as song (song.id)}
              {@const key = `song-${song.id}`}
              <li>
                <span class="text">
                  <a class="title" href={songHref(song.id)}>{song.title}</a>
                  <span class="sub">{song.artist || "Unknown artist"}{song.synced ? " · synced" : ""}</span>
                  {#if confirming === song.id}
                    <span class="result">Remove it everywhere and add it again?</span>
                  {/if}
                </span>
                {#if confirming === song.id}
                  <span class="pair">
                    <button type="button" class="button" onclick={() => (confirming = null)}>Keep</button>
                    <button
                      type="button"
                      class="button danger"
                      disabled={working !== ""}
                      onclick={() => {
                        confirming = null;
                        void run(key, () => Tools.ReaddSong(song.id));
                      }}>Re-add</button
                    >
                  </span>
                {:else}
                  <button type="button" class="button" disabled={working !== ""} onclick={() => (confirming = song.id)}
                    >{working === key ? "Adding…" : "Re-add"}</button
                  >
                {/if}
              </li>
            {/each}
          </ul>
          {#each Object.keys(results).filter((k) => k.startsWith("song-")) as key (key)}
            <p class="muted small" role="status">{told(key)}</p>
          {/each}
        {/if}
      </section>
    {/if}
  </main>
</div>

<style>
  main {
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 12px 0 max(32px, env(safe-area-inset-bottom));
  }

  p {
    margin: 0;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .intro {
    margin-bottom: 12px;
    font-size: 0.9375rem;
  }

  .small {
    margin-top: 8px;
    font-size: 0.875rem;
  }

  .search {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
  }

  .search input {
    flex: 1;
    min-width: 0;
    min-height: var(--tap);
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg);
    color: var(--fg);
    font: inherit;
    font-size: 1rem;
  }

  .button {
    flex: none;
  }

  .list {
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--surface);
    overflow: hidden;
  }

  .list li {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 60px;
    padding: 8px 12px 8px 16px;
  }

  .list li + li {
    border-top: 1px solid var(--surface-strong);
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title {
    font-weight: 600;
    color: var(--fg);
    text-decoration: none;
    overflow-wrap: anywhere;
  }

  .sub,
  .result {
    font-size: 0.875rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .result {
    color: var(--fg);
  }

  .pair {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
</style>
