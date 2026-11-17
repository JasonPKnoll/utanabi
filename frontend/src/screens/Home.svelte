<script lang="ts">
  import { AboutService, type About } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import {
    Service as Songs,
    type Summary,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { initial } from "../lib/lyrics";
  import { addHref, searchHref, songHref } from "../lib/route";
  import { sourceLabel } from "../lib/sources";

  let songs = $state<Summary[] | null>(null);
  let about = $state<About | null>(null);
  let failure = $state<AppError | null>(null);
  // Online search can be switched off in settings. Until the answer arrives, assume it's on, the usual case.
  let searchOn = $state(true);

  Songs.List()
    .then((list) => (songs = list ?? []))
    .catch((err: unknown) => (failure = toAppError(err)));

  // These two only decide whether a button and the version line show, so a failure keeps the library on screen:
  // search stays offered (the search screen reports its own errors), and the version line is left out.
  Songs.SearchSources()
    .then((names) => (searchOn = (names ?? []).length > 0))
    .catch(() => (searchOn = true));

  AboutService.Info()
    .then((info) => (about = info))
    .catch(() => (about = null));
</script>

<div class="screen">
  <header>
    <h1>
      <span class="mark" lang="ja">うたなび</span>
      <span class="latin">Utanabi</span>
    </h1>
  </header>

  <main>
    <h2>Your songs</h2>

    {#if failure}
      <p class="alert" role="alert">{failure.message}</p>
    {:else if songs === null}
      <p class="muted">Loading…</p>
    {:else if songs.length === 0}
      <div class="empty">
        <p class="empty-title">No songs yet</p>
        <p class="muted">
          {searchOn
            ? "Find a song to start learning from it, or add one with your own lyrics."
            : "Add a song with your own lyrics to start learning from it."}
        </p>
      </div>
    {:else}
      <ul>
        {#each songs as song (song.id)}
          <li>
            <a class="row" href={songHref(song.id)}>
              <span class="tile" aria-hidden="true">{initial(song.title)}</span>
              <span class="text">
                <span class="title">{song.title}</span>
                <span class="sub">{song.artist || "Unknown artist"} · {sourceLabel(song.source)}</span>
              </span>
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                aria-hidden="true"><polyline points="9 18 15 12 9 6" /></svg
              >
            </a>
          </li>
        {/each}
      </ul>
    {/if}

    {#if about}
      <p class="about" data-testid="about">{about.name} {about.version} · {about.platform}</p>
      {#if (about.credits ?? []).length > 0}
        <details class="credits">
          <summary>Credits</summary>
          {#each about.credits ?? [] as credit (credit.name)}
            <p><strong>{credit.name}.</strong> {credit.text}</p>
          {/each}
        </details>
      {/if}
    {/if}
  </main>

  <div class="actions">
    {#if !searchOn}
      <a class="button primary" href={addHref}>Add your own lyrics</a>
    {:else}
      <a class="button" href={addHref}>Add your own</a>
      <a class="button primary" href={searchHref}>
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          aria-hidden="true"><circle cx="11" cy="11" r="7" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg
        >
        Find a song
      </a>
    {/if}
  </div>
</div>

<style>
  header {
    padding: 16px 0 8px;
  }

  h1 {
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: 10px;
  }

  .mark {
    font-size: 1.6rem;
    font-weight: 700;
  }

  .latin {
    font-size: 0.85rem;
    font-weight: 500;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--muted);
  }

  main {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding-bottom: 16px;
  }

  h2 {
    margin: 16px 0 0;
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: 64px;
    padding: 8px 0;
    color: var(--fg);
    text-decoration: none;
  }

  .row:hover {
    color: var(--fg);
  }

  .row svg {
    flex: none;
    color: var(--muted);
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
    font-size: 1.05rem;
    font-weight: 600;
  }

  .sub {
    font-size: 0.85rem;
    color: var(--muted);
  }

  .about {
    margin: 24px 0 0;
    font-size: 0.8rem;
    color: var(--muted);
  }

  .credits {
    font-size: 0.8rem;
    color: var(--muted);
    overflow-wrap: anywhere;
  }

  .credits summary {
    display: flex;
    align-items: center;
    min-height: var(--tap);
    width: fit-content;
    color: var(--link);
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
  }

  .credits p {
    margin: 0 0 8px;
  }

  /* The secondary button takes only the room it needs, so "Find a song" stays on one line at 360px. */
  .actions .button:not(.primary) {
    flex: none;
  }

  .actions .button {
    white-space: nowrap;
  }
</style>
