<script lang="ts">
  import type { FoundTimes } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError } from "../lib/errors";

  // The song's word timing: how much of it is timed and by whom, tapping along to time it yourself, looking it up in
  // the timing databases, and clearing it. Without word times, each line's words are spread by their beats.
  let {
    timed,
    synced,
    byYou,
    sources,
    canTap,
    ontap,
    onfind,
    onclear,
  }: {
    // Lines with word times, of the synced lines, and how many of them you timed.
    timed: number;
    synced: number;
    byYou: number;
    // The timing databases that are on, such as "AMLL TTML DB".
    sources: string[];
    // Whether the lyrics follow the song now, so taps land at a time.
    canTap: boolean;
    ontap: () => void;
    onfind: () => Promise<FoundTimes>;
    onclear: () => Promise<void>;
  } = $props();

  let busy = $state(false);
  let note = $state("");
  let confirming = $state(false);

  function status(): string {
    if (synced === 0) {
      return "These lyrics have no line times, so their words can't be timed.";
    }
    if (timed === 0) {
      return "Not timed yet: each line's words are spread by their beats, which is a guess.";
    }
    const found = timed - byYou;
    const parts = [];
    if (byYou > 0) {
      parts.push(`${byYou} by you`);
    }
    if (found > 0) {
      parts.push(`${found} from ${sources[0] ?? "a timing database"}`);
    }
    return `${timed} of ${synced} lines timed: ${parts.join(", ")}.`;
  }

  async function find() {
    busy = true;
    note = "";
    try {
      const found = await onfind();
      note =
        found.lines > 0
          ? `Timed ${found.lines} of ${found.of} lines from ${found.source}.`
          : `${sources.join(" and ")} doesn't have this song yet. You can tap along to time it.`;
    } catch (err) {
      note = toAppError(err).message;
    } finally {
      busy = false;
    }
  }

  async function clear() {
    busy = true;
    note = "";
    try {
      await onclear();
      confirming = false;
    } catch (err) {
      note = toAppError(err).message;
    } finally {
      busy = false;
    }
  }
</script>

<p class="status">{status()}</p>

{#if synced > 0}
  <div class="buttons">
    <button type="button" class="button primary" disabled={!canTap || busy} onclick={ontap}>Tap along</button>
    {#if sources.length > 0}
      <button type="button" class="button" disabled={busy} onclick={find}>
        {busy && !confirming ? "Looking…" : "Find word times"}
      </button>
    {/if}
  </div>
  {#if !canTap}
    <p class="hint">Play the song with Spotify or YouTube, then tap along as each word starts.</p>
  {/if}
  {#if timed > 0}
    {#if confirming}
      <p class="confirm">Remove this song's word times, yours too?</p>
      <div class="buttons">
        <button type="button" class="button" disabled={busy} onclick={() => (confirming = false)}>Keep them</button>
        <button type="button" class="button danger" disabled={busy} onclick={clear}>Remove</button>
      </div>
    {:else}
      <button type="button" class="link" onclick={() => (confirming = true)}>Remove word times…</button>
    {/if}
  {/if}
{/if}
{#if note}<p class="note" role="status">{note}</p>{/if}

<style>
  .status,
  .hint,
  .note,
  .confirm {
    margin: 0 4px 10px;
    font-size: 0.9375rem;
  }

  .hint,
  .note {
    color: var(--muted);
    font-size: 0.875rem;
  }

  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 10px;
  }

  .buttons .button {
    flex: 1 1 auto;
  }

  .link {
    min-height: var(--tap);
    padding: 0 4px;
    border: 0;
    background: transparent;
    color: var(--danger);
    font: inherit;
    font-size: 0.9375rem;
    font-weight: 600;
    cursor: pointer;
  }
</style>
