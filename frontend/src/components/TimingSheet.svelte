<script lang="ts">
  import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { describeOffset, lyricsTime, nextLine } from "../lib/follow";
  import Sheet from "./Sheet.svelte";

  // Lines the lyrics up with Spotify's track, for lyrics timed for another version of the song. Nudge them by half a
  // second, or tap Now as the next line starts, which sets the offset from where the song is. Every change is yours
  // and saved for this song; Automatic goes back to the offset worked out from the two lengths.
  let {
    offsetMs,
    offsetSet,
    lines,
    now,
    onchange,
    onreset,
    onclose,
  }: {
    offsetMs: number;
    offsetSet: boolean;
    lines: Line[];
    // Where the song is on the track now, in ms.
    now: () => number;
    onchange: (offsetMs: number) => void;
    onreset: () => void;
    onclose: () => void;
  } = $props();

  const step = 500;
  const limit = 60000;

  // The line to tap Now for, kept up with the song.
  let next = $state(-1);
  let cue = $derived(lines[next]);
  $effect(() => {
    const tick = () => (next = nextLine(lines, lyricsTime(now(), offsetMs)));
    tick();
    const timer = window.setInterval(tick, 200);
    return () => window.clearInterval(timer);
  });

  function nudge(by: number) {
    onchange(Math.max(-limit, Math.min(limit, offsetMs + by)));
  }

  // The next line starts now: it's sung where the song is, so the offset is the difference, to a tenth of a second.
  function mark() {
    const start = cue?.startMs;
    if (start === null || start === undefined) {
      return;
    }
    onchange(Math.max(-limit, Math.min(limit, Math.round((now() - start) / 100) * 100)));
  }
</script>

<Sheet labelledby="timing-title" {onclose}>
  {#snippet children(close)}
    <header>
      <h2 id="timing-title">Lyrics timing</h2>
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

    <p class="muted">For lyrics timed for another version of the song, such as its video.</p>

    <div class="nudge">
      <button type="button" class="button" onclick={() => nudge(-step)} aria-label="Lines half a second earlier"
        >−0.5 s</button
      >
      <p class="value" aria-live="polite">
        <strong>Lines {describeOffset(offsetMs)}</strong>
        <span class="muted">{offsetSet ? "Set by you" : "Automatic"}</span>
      </p>
      <button type="button" class="button" onclick={() => nudge(step)} aria-label="Lines half a second later"
        >+0.5 s</button
      >
    </div>

    <h3>Tap as you hear it</h3>
    {#if cue}
      <p class="cue" lang="ja">{cue.text}</p>
      <button type="button" class="button primary now" onclick={mark}>Now</button>
    {:else}
      <p class="muted">The song is past its last line. Play it from earlier to tap along.</p>
    {/if}

    {#if offsetSet}
      <button type="button" class="button reset" onclick={onreset}>Back to automatic</button>
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

  h3 {
    margin: 24px 0 0;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  p {
    margin: 8px 0 0;
  }

  .nudge {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 16px;
  }

  .nudge .button {
    flex: none;
    min-width: 76px;
  }

  .value {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    margin: 0;
    text-align: center;
  }

  .value .muted {
    font-size: 0.8125rem;
  }

  .cue {
    min-height: 1.5em;
    font-size: 1.25rem;
    line-height: 1.5;
    text-align: center;
    overflow-wrap: anywhere;
  }

  .now,
  .reset {
    width: 100%;
    margin-top: 12px;
  }
</style>
