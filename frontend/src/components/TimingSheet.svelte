<script lang="ts">
  import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { describeOffset, lyricsTime, nextLine } from "../lib/follow";
  import Sheet from "./Sheet.svelte";

  // Lines the lyrics up with Spotify's track, for lyrics timed for another version of the song. Slide them earlier or
  // later, nudge them a tenth of a second at a time, or tap Now as the next line starts, which sets the offset from
  // where the song is. While you slide, the lyrics move with it (onpreview); letting go saves it (onchange). Every
  // change is yours and saved for this song; Reset goes back to the lyrics' own timing.
  let {
    offsetMs,
    offsetSet,
    lines,
    now,
    onpreview,
    onchange,
    onreset,
    onclose,
  }: {
    offsetMs: number;
    offsetSet: boolean;
    lines: Line[];
    // Where the song is on the track now, in ms.
    now: () => number;
    onpreview: (offsetMs: number) => void;
    onchange: (offsetMs: number) => void;
    onreset: () => void;
    onclose: () => void;
  } = $props();

  const limit = 60000;
  const step = 100;
  // The slider spans 5 seconds either way, the most a version's intro usually differs by, and widens to take in an
  // offset set further out, such as by tapping Now.
  const reach = 5000;
  let low = $derived(Math.min(-reach, Math.floor(offsetMs / 1000) * 1000));
  let high = $derived(Math.max(reach, Math.ceil(offsetMs / 1000) * 1000));

  function slid(event: Event & { currentTarget: HTMLInputElement }) {
    onpreview(Number(event.currentTarget.value));
  }

  function release(event: Event & { currentTarget: HTMLInputElement }) {
    onchange(Number(event.currentTarget.value));
  }

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

    <p class="value" aria-live="polite">
      <strong>Lines {describeOffset(offsetMs)}</strong>
      <span class="muted">{offsetSet ? "Set by you" : "As the lyrics are timed"}</span>
    </p>
    <div class="slider">
      <button
        type="button"
        class="icon-button"
        onclick={() => nudge(-step)}
        aria-label="Lines a tenth of a second earlier">−</button
      >
      <input
        type="range"
        min={low}
        max={high}
        {step}
        value={offsetMs}
        aria-label="Lyrics timing"
        aria-valuetext={`Lines ${describeOffset(offsetMs)}`}
        oninput={slid}
        onchange={release}
      />
      <button type="button" class="icon-button" onclick={() => nudge(step)} aria-label="Lines a tenth of a second later"
        >+</button
      >
    </div>
    <p class="ends muted" aria-hidden="true"><span>Earlier</span><span>Later</span></p>

    <h3>Tap as you hear it</h3>
    {#if cue}
      <p class="cue" lang="ja">{cue.text}</p>
      <button type="button" class="button primary now" onclick={mark}>Now</button>
    {:else}
      <p class="muted">The song is past its last line. Play it from earlier to tap along.</p>
    {/if}

    {#if offsetSet}
      <button type="button" class="button reset" onclick={onreset}>Reset to the lyrics' timing</button>
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

  .value {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin: 16px 0 0;
    text-align: center;
  }

  .value .muted {
    font-size: 0.8125rem;
  }

  .slider {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }

  .slider .icon-button {
    flex: none;
    font-size: 1.375rem;
    font-weight: 600;
    line-height: 1;
  }

  input[type="range"] {
    flex: 1;
    min-width: 0;
    height: var(--tap);
    margin: 0;
    accent-color: var(--link);
    cursor: pointer;
  }

  .ends {
    display: flex;
    justify-content: space-between;
    margin: 0;
    padding-inline: 52px;
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
