<script lang="ts">
  import type { ReadingAids } from "../lib/readingAids";
  import Sheet from "./Sheet.svelte";
  import ThemePicker from "./ThemePicker.svelte";

  // The song's display options in one place: the reading aids as switches, the app's theme, where the lyrics came
  // from, and deleting the song, so none of them take room from the lyrics.
  let {
    aids,
    source,
    deleting,
    ontoggle,
    ondelete,
    onclose,
  }: {
    aids: ReadingAids;
    source: string;
    deleting: boolean;
    ontoggle: (aid: keyof ReadingAids) => void;
    ondelete: () => void;
    onclose: () => void;
  } = $props();

  let confirmDelete = $state(false);

  const options: { aid: keyof ReadingAids; label: string; hint: string }[] = [
    { aid: "furigana", label: "Furigana", hint: "Readings over the kanji" },
    { aid: "romaji", label: "Romaji", hint: "Each line in Latin letters" },
    { aid: "words", label: "Words", hint: "English under each word" },
  ];
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

    <ul class="group">
      {#each options as option (option.aid)}
        <li>
          <button
            type="button"
            class="row"
            role="switch"
            aria-checked={aids[option.aid]}
            aria-labelledby={`aid-${option.aid}`}
            aria-describedby={`aid-${option.aid}-hint`}
            onclick={() => ontoggle(option.aid)}
          >
            <span class="text">
              <span class="label" id={`aid-${option.aid}`}>{option.label}</span>
              <span class="hint" id={`aid-${option.aid}-hint`}>{option.hint}</span>
            </span>
            <span class="switch" aria-hidden="true"><span class="knob"></span></span>
          </button>
        </li>
      {/each}
    </ul>

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

  .group {
    margin: 8px 0 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--bg);
    overflow: hidden;
  }

  .group li + li {
    border-top: 1px solid var(--surface-strong);
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

  .text {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .label {
    font-weight: 600;
  }

  .hint {
    font-size: 0.8125rem;
    color: var(--muted);
  }

  /* An on/off switch drawn from the button's aria-checked, so its look always matches what screen readers hear. */
  .switch {
    flex: none;
    position: relative;
    width: 46px;
    height: 28px;
    border-radius: 14px;
    background: var(--surface-strong);
    transition: background 0.15s;
  }

  .knob {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: var(--fg);
    transition: transform 0.15s;
  }

  .row[aria-checked="true"] .switch {
    background: var(--accent);
  }

  .row[aria-checked="true"] .knob {
    transform: translateX(18px);
    background: var(--on-accent);
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

  @media (prefers-reduced-motion: reduce) {
    .switch,
    .knob {
      transition: none;
    }
  }
</style>
