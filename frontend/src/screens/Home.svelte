<script lang="ts">
  import {
    Service as Songs,
    type Summary,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { initial } from "../lib/lyrics";
  import { placeholderSongs } from "../lib/placeholders";
  import { addHref, searchHref, settingsHref, songHref } from "../lib/route";
  import { sourceLabel } from "../lib/sources";

  let songs = $state<Summary[] | null>(null);
  let failure = $state<AppError | null>(null);
  // Online search can be switched off in settings. Until the answer arrives, assume it's on, the usual case.
  let searchOn = $state(true);

  Songs.List()
    .then((list) => (songs = list ?? []))
    .catch((err: unknown) => (failure = toAppError(err)));

  // This only decides whether the search bar shows, so a failure keeps it: the search screen reports its own errors.
  Songs.SearchSources()
    .then((names) => (searchOn = (names ?? []).length > 0))
    .catch(() => (searchOn = true));
</script>

<!-- A song's row. Without an id it's a stand-in for the skeleton, its text drawn as bars. -->
{#snippet row(title: string, sub: string, id?: number)}
  <svelte:element this={id === undefined ? "div" : "a"} class="row" href={id === undefined ? undefined : songHref(id)}>
    <span class="tile" aria-hidden="true">{id === undefined ? "" : initial(title)}</span>
    <span class="text">
      <span class="title"><span class:ph={id === undefined}>{title}</span></span>
      <span class="sub"><span class:ph={id === undefined}>{sub}</span></span>
    </span>
  </svelte:element>
{/snippet}

<div class="screen">
  <header class="topbar">
    <p class="mark"><span lang="ja">うたなび</span></p>
    <a class="icon-button" href={settingsHref} aria-label="Settings">
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
        ><circle cx="12" cy="12" r="3" /><path
          d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
        /></svg
      >
    </a>
  </header>

  <main>
    <h1 class="page-title">
      Library
      {#if songs && songs.length > 0}<span class="count">{songs.length}</span>{/if}
    </h1>

    {#if failure}
      <p class="alert" role="alert">{failure.message}</p>
    {:else if songs === null}
      <p class="visually-hidden" role="status">Loading your songs</p>
      <!-- Stand-in songs, drawn as rows like the real ones, as a skeleton in the library's shape. -->
      <ul class="songs skeleton-wait" aria-hidden="true" inert>
        {#each placeholderSongs as stand, i (i)}
          <li>{@render row(stand.title, `${stand.artist} · LRCLIB`)}</li>
        {/each}
      </ul>
    {:else if songs.length === 0}
      <div class="empty">
        <p class="empty-title">No songs yet</p>
        <p class="muted">
          {searchOn
            ? "Search for a song to start learning from it, or add one with your own lyrics."
            : "Add a song with your own lyrics to start learning from it."}
        </p>
      </div>
    {:else}
      <ul class="songs">
        {#each songs as song (song.id)}
          <li>
            {@render row(song.title, `${song.artist || "Unknown artist"} · ${sourceLabel(song.source)}`, song.id)}
          </li>
        {/each}
      </ul>
    {/if}
  </main>

  <div class="dock">
    {#if searchOn}
      <a class="search" href={searchHref}>
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
        Search songs or artists
      </a>
      <a class="add" href={addHref} aria-label="Add your own lyrics">
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg
        >
      </a>
    {:else}
      <a class="button primary" href={addHref}>Add your own lyrics</a>
    {/if}
  </div>
</div>

<style>
  .mark {
    flex: 1;
    margin: 0;
    padding: 0 8px;
    font-size: 1.25rem;
    font-weight: 800;
    letter-spacing: 0.02em;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding-bottom: 16px;
  }

  .count {
    margin-left: 6px;
    font-size: 1rem;
    font-weight: 600;
    color: var(--muted);
    vertical-align: middle;
  }

  .songs {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: 64px;
    margin: 0 -8px;
    padding: 6px 8px;
    border-radius: 14px;
    color: var(--fg);
    text-decoration: none;
  }

  .row:hover {
    color: var(--fg);
    background: var(--surface);
  }

  .tile {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border-radius: 12px;
    background: var(--surface-strong);
    color: var(--link);
    font-size: 1.25rem;
    font-weight: 700;
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .title,
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .title {
    font-weight: 600;
  }

  .sub {
    font-size: 0.8125rem;
    color: var(--muted);
  }

  /* Search and add, together in a slim bar within thumb reach. */
  .dock {
    position: sticky;
    bottom: 0;
    display: flex;
    gap: 10px;
    padding: 12px 0 max(16px, env(safe-area-inset-bottom));
    background: linear-gradient(to bottom, transparent, var(--bg) 20px);
  }

  .search {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 52px;
    padding: 0 18px;
    border-radius: 26px;
    background: var(--surface);
    box-shadow: 0 4px 20px rgb(0 0 0 / 18%);
    color: var(--muted);
    text-decoration: none;
    white-space: nowrap;
  }

  .search:hover {
    color: var(--fg);
  }

  .add {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 52px;
    height: 52px;
    border-radius: 26px;
    background: var(--accent);
    box-shadow: 0 4px 20px rgb(0 0 0 / 18%);
    color: var(--on-accent);
  }

  .add:hover {
    color: var(--on-accent);
    background: var(--accent-strong);
  }
</style>
