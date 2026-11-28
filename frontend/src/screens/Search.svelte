<script lang="ts">
  import { Service as Songs, type Match } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { aboutHref, addHref, homeHref, songHref } from "../lib/route";
  import { formatDuration } from "../lib/sources";

  let query = $state("");
  let searching = $state(false);
  let searched = $state("");
  let matches = $state<Match[]>([]);
  let unavailable = $state<string[]>([]);
  let failure = $state<AppError | null>(null);
  let adding = $state<string | null>(null);
  let input: HTMLInputElement | undefined = $state();

  // Opening the screen means you're about to type, so the field is ready.
  $effect(() => {
    input?.focus();
  });
  // The online sources that are on. Null until the app answers; empty when search is switched off in settings.
  let sourceNames = $state<string[] | null>(null);

  Songs.SearchSources()
    .then((names) => (sourceNames = names ?? []))
    .catch((err: unknown) => (failure = toAppError(err)));

  let queryError = $derived(failure?.code === "invalid_input" && failure.field === "query" ? failure.message : null);

  async function search(event: SubmitEvent) {
    event.preventDefault();
    if (searching) {
      return;
    }
    searching = true;
    failure = null;
    try {
      const result = await Songs.Search(query);
      matches = result.matches ?? [];
      unavailable = result.unavailable ?? [];
      searched = query.trim();
    } catch (err) {
      failure = toAppError(err);
    } finally {
      searching = false;
    }
  }

  async function add(match: Match) {
    if (adding !== null) {
      return;
    }
    adding = match.sourceRef;
    failure = null;
    try {
      const song = await Songs.AddFromSource(match.source, match.sourceRef);
      window.location.replace(songHref(song.id));
    } catch (err) {
      failure = toAppError(err);
      adding = null;
    }
  }

  function details(match: Match): string {
    return [match.artist || "Unknown artist", match.album, formatDuration(match.durationMs)]
      .filter((part) => part !== "")
      .join(" · ");
  }
</script>

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
    <h1 class="visually-hidden">Find a song</h1>
    {#if sourceNames === null || sourceNames.length > 0}
      <form onsubmit={search} role="search" novalidate>
        <label for="search-query" class="visually-hidden">Song title or artist</label>
        <div class="field" class:invalid={queryError !== null}>
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.25"
            stroke-linecap="round"
            aria-hidden="true"><circle cx="11" cy="11" r="7" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg
          >
          <input
            id="search-query"
            type="search"
            bind:this={input}
            bind:value={query}
            placeholder="Song title or artist"
            enterkeyhint="search"
            autocomplete="off"
            aria-invalid={queryError !== null}
            aria-describedby={queryError ? "search-query-error" : undefined}
          />
          <button class="go" type="submit" disabled={searching} aria-label="Search">
            {#if searching}
              <span class="spinner" aria-hidden="true"></span>
            {:else}
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2.5"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"><line x1="5" y1="12" x2="19" y2="12" /><polyline points="12 5 19 12 12 19" /></svg
              >
            {/if}
          </button>
        </div>
      </form>
    {:else}
      <div class="topbar-title"><p class="off-title">Find a song</p></div>
    {/if}
  </header>

  {#if queryError}
    <p id="search-query-error" class="error">{queryError}</p>
  {/if}

  {#if sourceNames !== null && sourceNames.length === 0}
    <main class="off">
      <p class="empty-title">Online lyrics search is turned off</p>
      <p class="muted">You can still add songs with your own lyrics.</p>
      <a class="button primary" href={addHref}>Add your own lyrics</a>
    </main>
  {:else}
    <main aria-live="polite">
      {#if failure && failure.code !== "invalid_input"}
        <p class="alert" role="alert">{failure.message}</p>
      {:else if failure?.code === "invalid_input" && failure.field !== "query"}
        <p class="alert" role="alert">{failure.message}</p>
      {/if}

      {#if unavailable.length > 0}
        <p class="notice">
          {unavailable.join(", ")}
          {unavailable.length === 1 ? "isn't" : "aren't"} answering right now. Try again in a moment, or add your own lyrics.
        </p>
      {/if}

      {#if searched !== "" && matches.length === 0 && unavailable.length === 0}
        <div class="empty">
          <p class="empty-title">No songs found for “{searched}”</p>
          <p class="muted">Try the title in Japanese or in rōmaji, or add the lyrics yourself.</p>
        </div>
      {:else if matches.length > 0}
        <ul class="results">
          {#each matches as match (match.source + match.sourceRef)}
            <li class="result">
              <div class="text">
                <span class="title">{match.title}</span>
                <span class="sub">
                  {details(match)}
                  {#if match.synced}<span class="chip accent">Synced</span>{/if}
                </span>
              </div>
              {#if match.songId > 0}
                <a class="saved" href={songHref(match.songId)} aria-label={`${match.title} is in your library`}>
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"><polyline points="5 12 10 17 19 8" /></svg
                  >
                </a>
              {:else}
                <button
                  type="button"
                  class="add"
                  onclick={() => add(match)}
                  disabled={adding !== null}
                  aria-label={`Add ${match.title} to your library`}
                >
                  {#if adding === match.sourceRef}
                    <span class="spinner" aria-hidden="true"></span>
                  {:else}
                    <svg
                      width="20"
                      height="20"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2.5"
                      stroke-linecap="round"
                      aria-hidden="true"
                      ><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg
                    >
                  {/if}
                </button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}

      {#if searched !== "" || unavailable.length > 0}
        <p class="fallback"><a href={addHref}>Can't find it? Add your own lyrics</a></p>
      {/if}
    </main>

    {#if sourceNames?.includes("LRCLIB")}
      <p class="footnote credit">
        Search by LRCLIB, a free community lyrics database · <a href={aboutHref}>Sources</a>
      </p>
    {/if}
  {/if}
</div>

<style>
  form {
    flex: 1;
    min-width: 0;
    padding-right: 8px;
  }

  .field {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 44px;
    padding: 0 4px 0 14px;
    border: 1px solid transparent;
    border-radius: 22px;
    background: var(--surface);
    color: var(--muted);
  }

  .field:focus-within {
    border-color: var(--link);
  }

  .field.invalid {
    border-color: var(--danger);
  }

  .field input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    border: 0;
    outline: none;
    background: transparent;
    color: var(--fg);
    font-size: 1rem;
  }

  .go {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border: 0;
    border-radius: 18px;
    background: var(--accent);
    color: var(--on-accent);
    cursor: pointer;
  }

  .go:disabled {
    opacity: 0.7;
  }

  .off-title {
    font-weight: 650;
  }

  .error {
    margin: 4px 4px 0;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 12px 0;
  }

  main.off {
    justify-content: center;
    align-items: center;
    gap: 8px;
    text-align: center;
  }

  main.off p {
    margin: 0;
  }

  main.off .button {
    flex: none;
    margin-top: 12px;
  }

  .notice {
    margin: 0;
    padding: 12px 14px;
    border-radius: 14px;
    background: var(--surface);
    color: var(--muted);
  }

  .results {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .result {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 64px;
    padding: 8px 0;
  }

  .result + .result {
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
    overflow-wrap: anywhere;
  }

  .sub {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    font-size: 0.8125rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .add,
  .saved {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: var(--tap);
    height: var(--tap);
    border: 0;
    border-radius: 22px;
    cursor: pointer;
  }

  .add {
    background: var(--surface-strong);
    color: var(--fg);
  }

  .add:hover {
    background: var(--accent);
    color: var(--on-accent);
  }

  .add:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .saved {
    background: transparent;
    color: var(--link);
  }

  .fallback {
    margin: 8px 0 0;
    text-align: center;
  }

  .fallback a {
    font-weight: 600;
    text-decoration: none;
  }

  .credit {
    padding-bottom: max(16px, env(safe-area-inset-bottom));
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation: none;
    }
  }
</style>
