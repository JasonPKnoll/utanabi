<script lang="ts">
  import { applyTheme, loadTheme, saveTheme, themes, type Theme } from "../lib/theme";

  // Auto, Light, or Dark, as three radio buttons drawn as one segmented control. The choice shows at once and is
  // remembered on this device.
  let theme = $state<Theme>(loadTheme());

  function choose(value: Theme) {
    theme = value;
    applyTheme(value);
    // When storage refuses, the theme still holds until the app closes.
    saveTheme(value);
  }
</script>

<fieldset class="picker">
  <legend class="visually-hidden">Theme</legend>
  {#each themes as option (option.value)}
    <label class:chosen={theme === option.value}>
      <input
        type="radio"
        name="theme"
        value={option.value}
        checked={theme === option.value}
        onchange={() => choose(option.value)}
      />
      {option.label}
    </label>
  {/each}
</fieldset>

<style>
  .picker {
    display: flex;
    gap: 4px;
    margin: 0;
    padding: 4px;
    border: 0;
    border-radius: 14px;
    background: var(--bg);
  }

  label {
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

  label.chosen {
    background: var(--surface-strong);
    color: var(--fg);
  }

  /* The radio itself is hidden but still focusable; its label shows the focus ring. */
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
