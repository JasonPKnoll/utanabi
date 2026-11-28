<script lang="ts">
  import type { ReadingAids } from "../lib/readingAids";

  // The reading aids as on/off switches: in the song's Display sheet, and in Settings. They're the same choice in
  // both places, remembered on this device. The group's background is --group-bg, so it can sit on a sheet or a page.
  let { aids, ontoggle }: { aids: ReadingAids; ontoggle: (aid: keyof ReadingAids) => void } = $props();

  const options: { aid: keyof ReadingAids; label: string; hint: string }[] = [
    { aid: "furigana", label: "Furigana", hint: "Readings over the kanji" },
    { aid: "romaji", label: "Romaji", hint: "Each line in Latin letters" },
    { aid: "words", label: "Words", hint: "English under each word" },
  ];
</script>

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

<style>
  .group {
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--group-bg, var(--bg));
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
    font: inherit;
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

  @media (prefers-reduced-motion: reduce) {
    .switch,
    .knob {
      transition: none;
    }
  }
</style>
