<script lang="ts">
  import { AboutService, type About } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import {
    Service as Songs,
    type Summary,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { initial } from "../lib/lyrics";
  import { addHref, songHref } from "../lib/route";

  let songs = $state<Summary[] | null>(null);
  let about = $state<About | null>(null);
  let failure = $state<AppError | null>(null);

  Songs.List()
    .then((list) => (songs = list ?? []))
    .catch((err: unknown) => (failure = toAppError(err)));

  AboutService.Info()
    .then((info) => (about = info))
    .catch((err: unknown) => (failure ??= toAppError(err)));
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
        <p class="muted">Add a song with your own lyrics to start learning from it.</p>
      </div>
    {:else}
      <ul>
        {#each songs as song (song.id)}
          <li>
            <a class="row" href={songHref(song.id)}>
              <span class="tile" aria-hidden="true">{initial(song.title)}</span>
              <span class="text">
                <span class="title">{song.title}</span>
                <span class="sub">{song.artist || "Unknown artist"} · your lyrics</span>
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
    {/if}
  </main>

  <div class="actions">
    <a class="button primary" href={addHref}>
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg
      >
      Add your own lyrics
    </a>
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
    color: var(--accent);
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

  .empty {
    padding: 28px 20px;
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
    font-size: 1.1rem;
    font-weight: 700;
  }

  .about {
    margin: 24px 0 0;
    font-size: 0.8rem;
    color: var(--muted);
  }
</style>
