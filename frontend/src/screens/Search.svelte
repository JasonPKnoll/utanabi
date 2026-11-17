<script lang="ts">
  import { Service as Songs, type Match } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { addHref, homeHref, songHref } from "../lib/route";
  import { formatDuration } from "../lib/sources";

  let query = $state("");
  let searching = $state(false);
  let searched = $state("");
  let matches = $state<Match[]>([]);
  let unavailable = $state<string[]>([]);
  let failure = $state<AppError | null>(null);
  let adding = $state<string | null>(null);
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
    <h1>Find a song</h1>
  </header>

  {#if sourceNames !== null && sourceNames.length === 0}
    <main class="off">
      <p class="empty-title">Online lyrics search is turned off</p>
      <p class="muted">You can still add songs with your own lyrics.</p>
    </main>
  {:else}
    <form onsubmit={search} role="search" novalidate>
      <label for="search-query" class="visually-hidden">Song title or artist</label>
      <div class="search-row">
        <input
          id="search-query"
          type="search"
          bind:value={query}
          placeholder="Song title or artist"
          enterkeyhint="search"
          autocomplete="off"
          aria-invalid={queryError !== null}
          aria-describedby={queryError ? "search-query-error" : undefined}
        />
        <button class="button primary" type="submit" disabled={searching}>{searching ? "Searching…" : "Search"}</button>
      </div>
      {#if queryError}
        <p id="search-query-error" class="error">{queryError}</p>
      {/if}
    </form>

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

      {#if searched !== ""}
        {#if matches.length === 0 && unavailable.length === 0}
          <div class="empty">
            <p class="empty-title">No songs found for “{searched}”</p>
            <p class="muted">Try the title in Japanese or in rōmaji, or add the lyrics yourself.</p>
          </div>
        {:else if matches.length > 0}
          <ul>
            {#each matches as match (match.source + match.sourceRef)}
              <li class="result">
                <div class="text">
                  <span class="title">{match.title}</span>
                  <span class="sub">{details(match)}</span>
                  <span class="tags">
                    {#if match.synced}<span class="tag">Synced</span>{/if}
                    <span class="tag plain">{match.sourceName}</span>
                  </span>
                </div>
                {#if match.songId > 0}
                  <a class="button small" href={songHref(match.songId)}>In library</a>
                {:else}
                  <button
                    type="button"
                    class="button primary small"
                    onclick={() => add(match)}
                    disabled={adding !== null}
                    aria-label={`Add ${match.title} to your library`}
                  >
                    {adding === match.sourceRef ? "Adding…" : "Add"}
                  </button>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </main>
  {/if}

  <div class="actions">
    {#if sourceNames !== null && sourceNames.length === 0}
      <a class="button primary" href={addHref}>Add your own lyrics</a>
    {:else}
      <a class="button" href={addHref}>Can't find it? Add your own lyrics</a>
    {/if}
  </div>
  {#if sourceNames?.includes("LRCLIB")}
    <p class="credit muted">Lyrics search by LRCLIB (lrclib.net), a free, community-run lyrics database.</p>
  {/if}
</div>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 0 12px;
    margin-left: -8px;
  }

  h1 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 700;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .search-row {
    display: flex;
    gap: 8px;
  }

  .search-row input {
    flex: 1;
    min-width: 0;
    box-sizing: border-box;
    min-height: 52px;
    padding: 0 16px;
    border: 1px solid var(--border);
    border-radius: 26px;
    background: var(--surface);
    color: var(--fg);
    font-size: 1rem;
  }

  .search-row input[aria-invalid="true"] {
    border-color: var(--danger);
  }

  .search-row .button {
    flex: none;
  }

  .error {
    margin: 0;
    font-size: 0.9rem;
    color: var(--danger);
  }

  .error::first-letter {
    text-transform: uppercase;
  }

  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 0;
  }

  main.off {
    justify-content: center;
    align-items: center;
    gap: 6px;
    text-align: center;
  }

  main.off p {
    margin: 0;
  }

  .notice {
    margin: 0;
    padding: 12px 14px;
    border-radius: 12px;
    background: var(--surface);
    color: var(--muted);
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .result {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title {
    font-size: 1.05rem;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .sub {
    font-size: 0.85rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 2px;
  }

  .tag {
    padding: 1px 8px;
    border-radius: 999px;
    background: var(--surface-strong);
    color: var(--link);
    font-size: 0.75rem;
    font-weight: 600;
  }

  .tag.plain {
    color: var(--muted);
  }

  .button.small {
    flex: none;
    min-height: var(--tap);
    padding: 0 16px;
    font-size: 0.95rem;
  }

  .empty {
    padding: 24px 20px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    text-align: center;
  }

  .empty p {
    margin: 0;
  }

  .empty-title {
    margin-bottom: 4px !important;
    font-weight: 700;
    overflow-wrap: anywhere;
  }

  .actions .button {
    font-weight: 600;
  }

  .credit {
    margin: 0;
    padding-bottom: max(12px, env(safe-area-inset-bottom));
    font-size: 0.8rem;
    text-align: center;
  }
</style>
