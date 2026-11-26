<script lang="ts">
  import {
    Service as Songs,
    type Meanings,
    type RubyPart,
    type Word,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { aboutHref } from "../lib/route";
  import { hasMeanings, lemmaLine, splitMeanings } from "../lib/words";
  import Sheet from "./Sheet.svelte";

  let {
    word,
    ruby,
    onclose,
  }: {
    word: Word;
    ruby: RubyPart[];
    onclose: () => void;
  } = $props();

  let meanings = $state<Meanings | null>(null);
  let failure = $state<AppError | null>(null);

  let split = $derived(meanings ? splitMeanings(meanings.senses ?? [], word, meanings.entryId) : null);
  let steps = $derived(word.steps ?? []);

  $effect(() => {
    if (!hasMeanings(word)) {
      return;
    }
    meanings = null;
    failure = null;
    Songs.LookupWord(word.lemma, word.lemmaReading, word.entryId)
      .then((found) => (meanings = found))
      .catch((err: unknown) => (failure = toAppError(err)));
  });
</script>

<Sheet labelledby="word-sheet-title" {onclose}>
  {#snippet children(close)}
    <header>
      <!-- prettier-ignore -->
      <h2 id="word-sheet-title" lang="ja">{#each ruby as part, i (i)}{#if part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}</h2>
      <button type="button" class="icon-button close" aria-label="Close" onclick={close}>
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          aria-hidden="true"><line x1="6" y1="6" x2="18" y2="18" /><line x1="18" y1="6" x2="6" y2="18" /></svg
        >
      </button>
    </header>
    <p class="lemma"><span lang="ja">{lemmaLine(word)}</span></p>

    {#if word.form}
      <p class="form" lang="ja">{word.form}</p>
    {/if}

    {#if word.grammar}
      <section>
        <h3>{word.grammar.title} <span class="pattern" lang="ja">{word.grammar.pattern}</span></h3>
        <p>{word.grammar.meaning}</p>
        <p class="muted">{word.grammar.explanation}</p>
      </section>
    {/if}

    {#if hasMeanings(word)}
      <section aria-live="polite">
        {#if failure}
          <p class="alert" role="alert">{failure.message}</p>
        {:else if meanings === null}
          <p class="muted">Looking it up…</p>
        {:else if !meanings.installed}
          <p class="note">Meanings aren't installed</p>
          <p class="muted">Install the dictionary to see what this word means.</p>
        {:else if split && (split.used || split.others.length > 0)}
          {#if split.used}
            <h3>In this song</h3>
            <div class="meaning used">
              <p class="glosses">{(split.used.glosses ?? []).join("; ")}</p>
              <p class="tags">{[...(split.used.partsOfSpeech ?? []), ...(split.used.notes ?? [])].join(" · ")}</p>
            </div>
          {/if}
          {#if split.others.length > 0}
            <h3>{split.used ? "Other meanings" : "Meanings"}</h3>
            <ol class="meanings">
              {#each split.others as sense (sense.index)}
                <li class="meaning">
                  <p class="glosses">{(sense.glosses ?? []).join("; ")}</p>
                  <p class="tags">{[...(sense.partsOfSpeech ?? []), ...(sense.notes ?? [])].join(" · ")}</p>
                </li>
              {/each}
            </ol>
          {/if}
        {:else}
          <p class="muted">This word isn't in the dictionary.</p>
        {/if}
      </section>
    {:else if !word.grammar}
      <p class="muted">A grammar word that joins the words around it.</p>
    {/if}

    {#if steps.length > 0}
      <section>
        <h3>How it's conjugated</h3>
        <ol class="steps">
          {#each steps as step, i (i)}
            <li>
              <span class="step-title">{step.title} <span class="pattern" lang="ja">{step.pattern}</span></span>
              <span class="muted">{step.meaning}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}

    {#if meanings?.installed}
      <p class="footnote">Definitions from JMdict · <a href={aboutHref}>Sources</a></p>
    {/if}
  {/snippet}
</Sheet>

<style>
  header {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding-top: 12px;
  }

  h2 {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 2rem;
    font-weight: 700;
    line-height: 2.1;
    overflow-wrap: anywhere;
  }

  h2 rt {
    font-size: max(0.45em, 12px);
    font-weight: 400;
    color: var(--muted);
  }

  .close {
    border: 0;
    background: transparent;
    cursor: pointer;
  }

  .lemma {
    margin: 0 0 12px;
    color: var(--muted);
  }

  .form {
    margin: 0 0 16px;
  }

  section {
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

  .pattern {
    text-transform: none;
    letter-spacing: 0;
    font-weight: 400;
  }

  section p {
    margin: 0 0 8px;
  }

  .note {
    font-weight: 700;
  }

  .meanings,
  .steps {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .meaning {
    padding: 10px 0;
    border-top: 1px solid var(--border);
  }

  .meaning.used {
    padding: 12px 14px;
    margin-bottom: 16px;
    border: 1px solid var(--link);
    border-radius: 12px;
  }

  .glosses {
    margin: 0;
    font-weight: 600;
  }

  .tags {
    margin: 4px 0 0;
    font-size: 0.9rem;
    color: var(--muted);
  }

  .steps li {
    display: flex;
    flex-direction: column;
    padding: 8px 0;
    border-top: 1px solid var(--border);
  }

  .step-title {
    font-weight: 600;
  }
</style>
