<script lang="ts">
  import type { Video } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/youtube";
  import { toAppError } from "../lib/errors";
  import Sheet from "./Sheet.svelte";

  // Picks the song's video: paste a link from YouTube or YouTube Music, or, when the server can search, have it find
  // one again. The choice is yours alone.
  let {
    video,
    searchable,
    query,
    onsave,
    onsearch,
    switchTo = "",
    onswitch,
    onclose,
  }: {
    video: Video | null;
    searchable: boolean;
    // What to search YouTube for: the song's artist and title.
    query: string;
    onsave: (link: string) => Promise<void>;
    onsearch: () => Promise<void>;
    // The other player the song can play along with, such as "Spotify", and switching to it; none when there's none.
    switchTo?: string;
    onswitch?: () => void;
    onclose: () => void;
  } = $props();

  let link = $state("");
  let busy = $state(false);
  let note = $state("");
  const linkId = $props.id();

  async function run(fn: () => Promise<void>, close: () => void) {
    busy = true;
    note = "";
    try {
      await fn();
      close();
    } catch (err) {
      note = toAppError(err).message;
    } finally {
      busy = false;
    }
  }
</script>

<Sheet labelledby="video-title" {onclose}>
  {#snippet children(close)}
    <header>
      <h2 id="video-title">The song's video</h2>
      <button type="button" class="icon-button" aria-label="Close" onclick={close}>
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          aria-hidden="true"><line x1="6" y1="6" x2="18" y2="18" /><line x1="18" y1="6" x2="6" y2="18" /></svg
        >
      </button>
    </header>

    {#if video?.id}
      <p class="now">
        <strong>{video.title || "A video you added"}</strong>
        {#if video.channel}<span class="muted">{video.channel}</span>{/if}
      </p>
    {:else}
      <p class="muted">No video yet.</p>
    {/if}

    <a
      class="button youtube-search"
      href={`https://www.youtube.com/results?search_query=${encodeURIComponent(query)}`}
      target="_blank"
      rel="noopener noreferrer">Search on YouTube</a
    >
    <p class="muted small">Find the video, tap Share, then Copy link, and paste it here.</p>

    <form
      onsubmit={(event) => {
        event.preventDefault();
        void run(() => onsave(link), close);
      }}
    >
      <label for={linkId}>A link from YouTube or YouTube Music</label>
      <input
        id={linkId}
        type="url"
        inputmode="url"
        autocomplete="off"
        placeholder="https://youtu.be/…"
        bind:value={link}
        disabled={busy}
      />
      <button type="submit" class="button primary" disabled={busy || link.trim() === ""}>Use this video</button>
    </form>

    {#if searchable}
      <button type="button" class="button search" disabled={busy} onclick={() => run(onsearch, close)}>
        Find it on YouTube again
      </button>
    {/if}
    {#if note}<p class="alert" role="alert">{note}</p>{/if}
    {#if switchTo && onswitch}
      <button
        type="button"
        class="button search"
        onclick={() => {
          close();
          onswitch();
        }}>Play along with {switchTo} instead</button
      >
    {/if}
  {/snippet}
</Sheet>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0 4px;
  }

  h2 {
    margin: 0;
    font-size: 1.125rem;
    font-weight: 650;
  }

  p {
    margin: 8px 0 0;
  }

  .now {
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-wrap: anywhere;
  }

  .now .muted {
    font-size: 0.875rem;
  }

  .youtube-search {
    width: 100%;
    margin-top: 16px;
  }

  .small {
    font-size: 0.875rem;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 16px;
  }

  label {
    font-size: 0.875rem;
    font-weight: 600;
  }

  input {
    min-height: var(--tap);
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg);
    color: var(--fg);
    font: inherit;
    font-size: 1rem;
  }

  .search {
    width: 100%;
    margin-top: 12px;
  }
</style>
