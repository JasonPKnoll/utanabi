<script lang="ts">
  import type { Word } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { finishSung, scriptOf, typeSung, type Script } from "../lib/sung";

  // In the word sheet: the reading the song's lyrics give the word, when it isn't the dictionary's (gikun, such as
  // 羅針盤 with コンパス over it). Plain-text lyrics lose the lyricist's furigana, so you add it back here. It then
  // shows over the word and in the romaji, while the meaning stays the written word's. It's yours alone, even on a
  // song others have added too.
  let { word, onsave }: { word: Word; onsave: (reading: string) => Promise<void> } = $props();

  let editing = $state(false);
  let script = $state<Script>("katakana");
  let text = $state("");
  let saving = $state(false);
  let failure = $state<AppError | null>(null);
  let input: HTMLInputElement | undefined = $state();

  $effect(() => {
    if (editing) {
      input?.focus();
    }
  });

  function edit() {
    script = scriptOf(word.sung);
    text = word.sung;
    failure = null;
    editing = true;
  }

  function type(event: Event & { currentTarget: HTMLInputElement }) {
    text = typeSung(event.currentTarget.value, script);
  }

  function choose(value: Script) {
    script = value;
    text = typeSung(text, value);
  }

  async function save(reading: string) {
    saving = true;
    failure = null;
    try {
      await onsave(reading);
      editing = false;
    } catch (err) {
      failure = toAppError(err);
    } finally {
      saving = false;
    }
  }

  function submit(event: SubmitEvent) {
    event.preventDefault();
    const reading = finishSung(text, script);
    if (reading === "") {
      failure = { code: "invalid_input", field: "reading", message: "Type how it's sung, in kana or romaji." };
      return;
    }
    void save(reading);
  }
</script>

<section class="sung">
  {#if editing}
    <form onsubmit={submit} novalidate>
      <label for="sung-reading">How it's sung</label>
      <p id="sung-hint" class="muted hint">In kana, or in romaji, which turns into kana: konpasu is コンパス.</p>
      <fieldset class="scripts">
        <legend class="visually-hidden">Write it in</legend>
        <label class:chosen={script === "katakana"}>
          <input
            type="radio"
            name="sung-script"
            checked={script === "katakana"}
            onchange={() => choose("katakana")}
          />カタカナ
        </label>
        <label class:chosen={script === "hiragana"}>
          <input
            type="radio"
            name="sung-script"
            checked={script === "hiragana"}
            onchange={() => choose("hiragana")}
          />ひらがな
        </label>
      </fieldset>
      <input
        id="sung-reading"
        class="field"
        bind:this={input}
        value={text}
        oninput={type}
        lang="ja"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        enterkeyhint="done"
        placeholder={script === "katakana" ? "コンパス" : "ほし"}
        aria-describedby={failure ? "sung-hint sung-error" : "sung-hint"}
        aria-invalid={failure !== null}
      />
      {#if failure}
        <p id="sung-error" class="error" role="alert">{failure.message}</p>
      {/if}
      <div class="row">
        <button type="button" class="button" disabled={saving} onclick={() => (editing = false)}>Cancel</button>
        <button type="submit" class="button primary" disabled={saving}>{saving ? "Saving…" : "Save"}</button>
      </div>
    </form>
  {:else if word.sung}
    <h3>Sung as</h3>
    <div class="card">
      <p class="reading" lang="ja">{word.sung}</p>
      <p class="muted">{word.sungRomaji} · the furigana and romaji follow how it's sung</p>
    </div>
    {#if failure}
      <p class="error" role="alert">{failure.message}</p>
    {/if}
    <div class="row">
      <button type="button" class="button" disabled={saving} onclick={() => save("")}>Remove</button>
      <button type="button" class="button" disabled={saving} onclick={edit}>Change</button>
    </div>
  {:else}
    <button type="button" class="link" onclick={edit}>Sung differently from how it's written?</button>
  {/if}
</section>

<style>
  .sung {
    margin-top: 16px;
  }

  h3 {
    margin: 0 0 8px;
    font-size: 0.8rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }

  p {
    margin: 0;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  form > label {
    font-weight: 650;
  }

  .hint {
    font-size: 0.875rem;
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-radius: 12px;
  }

  .reading {
    font-size: 1.5rem;
    font-weight: 700;
  }

  .card .muted {
    font-size: 0.875rem;
  }

  .scripts {
    display: flex;
    gap: 4px;
    margin: 0;
    padding: 4px;
    border: 0;
    border-radius: 14px;
    background: var(--bg);
  }

  .scripts label {
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

  .scripts label.chosen {
    background: var(--surface-strong);
    color: var(--fg);
  }

  .scripts label:has(:focus-visible) {
    outline: 3px solid var(--link);
    outline-offset: 2px;
  }

  .scripts input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .field {
    min-height: 48px;
    padding: 0 14px;
    border: 2px solid var(--border);
    border-radius: 12px;
    background: var(--bg);
    color: var(--fg);
    font-size: 1.25rem;
  }

  .field:focus {
    border-color: var(--link);
    outline: none;
  }

  .row {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }

  .link {
    min-height: var(--tap);
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--link);
    font-weight: 600;
    text-align: left;
    text-decoration: underline;
    text-underline-offset: 3px;
    cursor: pointer;
  }
</style>
