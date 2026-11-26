<script lang="ts">
  import {
    applyLook,
    loadMode,
    loadPalette,
    modes,
    palettes,
    saveMode,
    savePalette,
    type Mode,
    type Palette,
  } from "../lib/theme";

  // The palette, as swatches, and the mode, as a segmented control: both radio buttons, so they work with a tap and a
  // keyboard. A choice shows at once and is remembered on this device.
  let palette = $state<Palette>(loadPalette());
  let mode = $state<Mode>(loadMode());

  function choosePalette(value: Palette) {
    palette = value;
    applyLook(palette, mode);
    // When storage refuses, the choice still holds until the app closes.
    savePalette(value);
  }

  function chooseMode(value: Mode) {
    mode = value;
    applyLook(palette, mode);
    saveMode(value);
  }
</script>

<fieldset class="palettes">
  <legend class="visually-hidden">Palette</legend>
  {#each palettes as option (option.value)}
    <label class="palette" class:chosen={palette === option.value}>
      <input
        type="radio"
        name="palette"
        value={option.value}
        checked={palette === option.value}
        onchange={() => choosePalette(option.value)}
      />
      <!-- The palette's own colours, dark on the left and light on the right, whatever the app shows now. -->
      <span class="swatch" data-palette={option.value} aria-hidden="true">
        <span class="half dark"><span class="dot"></span><span class="dot link"></span></span>
        <span class="half light"><span class="dot"></span><span class="dot link"></span></span>
      </span>
      <span class="name">{option.label}</span>
    </label>
  {/each}
</fieldset>

<fieldset class="modes">
  <legend class="visually-hidden">Mode</legend>
  {#each modes as option (option.value)}
    <label class="mode" class:chosen={mode === option.value}>
      <input
        type="radio"
        name="mode"
        value={option.value}
        checked={mode === option.value}
        onchange={() => chooseMode(option.value)}
      />
      {option.label}
    </label>
  {/each}
</fieldset>

<style>
  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
  }

  .palettes {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

  .palette {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    min-height: var(--tap);
    padding: 8px 4px;
    border-radius: 14px;
    background: var(--bg);
    color: var(--muted);
    font-size: 0.8125rem;
    font-weight: 600;
    text-align: center;
    cursor: pointer;
  }

  .palette.chosen {
    box-shadow: inset 0 0 0 2px var(--link);
    color: var(--fg);
  }

  .swatch {
    display: flex;
    width: 100%;
    max-width: 72px;
    height: 28px;
    border-radius: 8px;
    overflow: hidden;
    box-shadow: inset 0 0 0 1px var(--border);
  }

  .half {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
  }

  .dark {
    background: var(--d-bg);
  }

  .light {
    background: var(--l-bg);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }

  .dark .dot {
    background: var(--d-accent);
  }

  .dark .dot.link {
    background: var(--d-link);
  }

  .light .dot {
    background: var(--l-accent);
  }

  .light .dot.link {
    background: var(--l-link);
  }

  .modes {
    display: flex;
    gap: 4px;
    margin-top: 12px;
    padding: 4px;
    border-radius: 14px;
    background: var(--bg);
  }

  .mode {
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

  .mode.chosen {
    background: var(--surface-strong);
    color: var(--fg);
  }

  /* The radios are hidden but still focusable; their labels show the focus ring. */
  input {
    position: absolute;
    opacity: 0;
    width: 1px;
    height: 1px;
  }

  label:has(input:focus-visible) {
    outline: 3px solid var(--link);
    outline-offset: 2px;
  }
</style>
