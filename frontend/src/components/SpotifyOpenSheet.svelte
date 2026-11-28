<script lang="ts">
  import Sheet from "./Sheet.svelte";

  // Asks you to open Spotify when none of your devices has it open, so the app has nowhere to play. Its button is the
  // track's web link, which a phone opens in the Spotify app with no "Open in Spotify?" prompt from the browser, as a
  // spotify: link always gets. In a new tab, so a browser without the app keeps this page and opens the web player.
  let {
    link,
    onopen,
    onclose,
  }: {
    link: string;
    // Spotify is being opened: coming back to this page starts the song.
    onopen: () => void;
    onclose: () => void;
  } = $props();
</script>

<Sheet labelledby="spotify-open-title" {onclose}>
  {#snippet children(close)}
    <header>
      <h2 id="spotify-open-title">Open Spotify to play</h2>
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

    <p>Spotify isn't open on any of your devices, so there's nowhere to play yet.</p>
    <p class="muted">Open it, then come back here, and the song starts.</p>

    <div class="choices">
      <button type="button" class="button" onclick={close}>Not now</button>
      <a
        class="button primary"
        href={link}
        target="_blank"
        rel="noopener noreferrer"
        onclick={() => {
          onopen();
          close();
        }}>Open Spotify</a
      >
    </div>
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

  .choices {
    display: flex;
    gap: 12px;
    margin-top: 20px;
  }

  .choices .button {
    flex: 1;
  }
</style>
