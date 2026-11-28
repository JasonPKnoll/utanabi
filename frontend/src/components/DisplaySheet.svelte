<script lang="ts">
  import type { Snippet } from "svelte";
  import type { Player } from "../lib/players";
  import type { ReadingAids } from "../lib/readingAids";
  import ReadingAidSwitches from "./ReadingAidSwitches.svelte";
  import Sheet from "./Sheet.svelte";
  import ThemePicker from "./ThemePicker.svelte";

  // The song's display options in one place: the reading aids as switches, what the lyrics play along with (Spotify
  // or the song's YouTube video, when both are there), its word timing, the app's theme, where the lyrics came from,
  // and deleting the song, so none of them take room from the lyrics.
  let {
    aids,
    source,
    deleting,
    players = [],
    player = null,
    onplayer,
    ontoggle,
    ondelete,
    onclose,
    timing,
  }: {
    aids: ReadingAids;
    source: string;
    deleting: boolean;
    players?: Player[];
    player?: Player | null;
    onplayer?: (player: Player) => void;
    ontoggle: (aid: keyof ReadingAids) => void;
    ondelete: () => void;
    onclose: () => void;
    // The song's word timing, given the sheet's close, for actions that leave the sheet.
    timing?: Snippet<[() => void]>;
  } = $props();

  const playerNames: Record<Player, string> = { spotify: "Spotify", youtube: "YouTube" };

  let confirmDelete = $state(false);
</script>

<Sheet labelledby="display-title" {onclose}>
  {#snippet children(close)}
    <header>
      <h2 id="display-title">Display</h2>
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

    <div class="aids"><ReadingAidSwitches {aids} {ontoggle} /></div>

    {#if players.length > 1}
      <h3 class="section">Play along with</h3>
      <fieldset class="players">
        <legend class="visually-hidden">Play along with</legend>
        {#each players as p (p)}
          <label class="player" class:chosen={player === p}>
            <input type="radio" name="player" value={p} checked={player === p} onchange={() => onplayer?.(p)} />
            {playerNames[p]}
          </label>
        {/each}
      </fieldset>
    {/if}

    {#if timing}
      <h3 class="section">Word timing</h3>
      {@render timing(close)}
    {/if}

    <h3 class="section">Theme</h3>
    <ThemePicker />

    {#if source}
      <p class="source">Lyrics from {source}</p>
    {/if}

    <div class="danger-zone">
      {#if confirmDelete}
        <p class="confirm">Delete this song and the words saved from it?</p>
        <div class="confirm-actions">
          <button type="button" class="button" onclick={() => (confirmDelete = false)} disabled={deleting}>
            Keep it
          </button>
          <button type="button" class="button danger" onclick={ondelete} disabled={deleting}>
            {deleting ? "Deleting…" : "Delete song"}
          </button>
        </div>
      {:else}
        <button type="button" class="row delete" onclick={() => (confirmDelete = true)}>Delete song…</button>
      {/if}
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

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    min-height: 60px;
    padding: 8px 16px;
    border: 0;
    background: transparent;
    color: var(--fg);
    text-align: left;
    cursor: pointer;
  }

  .aids {
    margin-top: 8px;
  }

  .players {
    display: flex;
    gap: 4px;
    margin: 0;
    padding: 4px;
    border: 0;
    border-radius: 14px;
    background: var(--bg);
  }

  .player {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: var(--tap);
    border-radius: 10px;
    color: var(--muted);
    font-weight: 600;
    cursor: pointer;
  }

  .player.chosen {
    background: var(--surface-strong);
    color: var(--fg);
  }

  .player input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .player:has(input:focus-visible) {
    outline: 2px solid var(--link);
    outline-offset: 2px;
  }

  .section {
    margin: 18px 4px 8px;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .source {
    margin: 14px 4px 0;
    font-size: 0.8125rem;
    color: var(--muted);
  }

  .danger-zone {
    margin-top: 16px;
  }

  .delete {
    justify-content: center;
    min-height: 52px;
    border-radius: 16px;
    background: var(--bg);
    color: var(--danger);
    font-weight: 600;
  }

  .confirm {
    margin: 0 0 12px;
    text-align: center;
  }

  .confirm-actions {
    display: flex;
    gap: 12px;
  }
</style>
