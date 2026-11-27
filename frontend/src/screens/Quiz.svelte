<script lang="ts">
  import { toHiragana } from "wanakana";

  import {
    Service as Study,
    type Card,
    type Deck,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/study";
  import { meaningIsRight, readingIsRight, typedReading } from "../lib/answers";
  import { toAppError, type AppError } from "../lib/errors";
  import { studyHref } from "../lib/route";
  import { lineParts } from "../lib/studyList";
  import { afterAnswer, startQueue } from "../lib/studyQueue";

  // Flashcards for one song, asked the way WaniKani asks: the word, then its reading typed in kana, then its meaning
  // typed in English. A card is right when every question on it is, and only then does its streak grow.
  let { songId }: { songId: number } = $props();

  type Question = "reading" | "meaning";

  let deck = $state<Deck | null>(null);
  let failure = $state<AppError | null>(null);
  let queue = $state<number[]>([]);
  let question = $state<Question>("reading");
  let answer = $state("");
  // Each question's result on the card being asked. checked: the current question has been answered. done: every
  // question has, and the back of the card shows.
  let marks = $state<Partial<Record<Question, boolean>>>({});
  let checked = $state(false);
  let done = $state(false);
  let saving = $state(false);
  let input: HTMLInputElement | undefined = $state();

  let cards = $derived((deck?.cards ?? []).filter((c) => questionsOf(c).length > 0));
  let card = $derived(cards.find((c) => c.id === queue[0]));
  let learned = $derived(cards.filter((c) => c.progress.learned).length);
  let goal = $derived(deck?.goal ?? 3);
  let allRight = $derived(Object.values(marks).every(Boolean));
  let streakAfter = $derived(card ? (allRight ? card.progress.streak + 1 : 0) : 0);
  let current = $derived(question === "reading" ? marks.reading : marks.meaning);

  $effect(() => {
    Study.Deck(songId)
      .then((d) => {
        deck = d;
        begin(false);
      })
      .catch((err: unknown) => (failure = toAppError(err)));
  });

  // The one answer field stays focused for the whole session, even while an answer shows, so the phone's keyboard
  // stays up from card to card: iOS only raises it again for a focus inside a tap.
  $effect(() => {
    void card?.id;
    input?.focus();
  });

  function questionsOf(c: Card): Question[] {
    const asked: Question[] = [];
    if (c.askReading) {
      asked.push("reading");
    }
    if ((c.meanings ?? []).length > 0) {
      asked.push("meaning");
    }
    return asked;
  }

  function begin(all: boolean) {
    queue = startQueue(
      cards.map((c) => ({ id: c.id, learned: c.progress.learned })),
      all,
    );
    startCard();
  }

  function startCard() {
    const first = queue[0] !== undefined ? cards.find((c) => c.id === queue[0]) : undefined;
    question = first ? (questionsOf(first)[0] ?? "meaning") : "reading";
    marks = {};
    answer = "";
    checked = false;
    done = false;
  }

  function type(event: Event & { currentTarget: HTMLInputElement }) {
    if (checked || done) {
      // An answer is showing: typing changes nothing, and Enter goes on.
      event.currentTarget.value = answer;
      return;
    }
    const value = event.currentTarget.value;
    // WanaKana's kana input: romaji turns into hiragana as it's typed, and a lone n waits for the next letter.
    answer = question === "reading" ? toHiragana(value, { IMEMode: true }) : value;
  }

  function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!card || saving) {
      return;
    }
    if (done) {
      void next(card);
      return;
    }
    if (checked) {
      nextQuestion(card);
      return;
    }
    if (answer.trim() === "") {
      return;
    }
    if (question === "reading") {
      answer = typedReading(answer);
      marks.reading = readingIsRight(answer, card.readings ?? []);
    } else {
      marks.meaning = meaningIsRight(answer, card.meanings ?? []);
    }
    checked = true;
    if (current) {
      nextQuestion(card);
    }
  }

  function nextQuestion(c: Card) {
    const asked = questionsOf(c);
    const following = asked[asked.indexOf(question) + 1];
    if (following) {
      question = following;
      answer = "";
      checked = false;
    } else {
      done = true;
    }
  }

  // For a typo or an answer the check didn't know: counts the current question as right.
  function countAsRight() {
    marks[question] = true;
  }

  async function next(c: Card) {
    saving = true;
    try {
      const progress = await Study.Answer(c.id, allRight);
      c.progress = progress;
      queue = afterAnswer(queue, allRight, progress.learned);
      startCard();
    } catch (err) {
      failure = toAppError(err);
    } finally {
      saving = false;
    }
  }
</script>

<div class="screen">
  <header class="topbar">
    <a class="icon-button" href={studyHref(songId)} aria-label="Back to the word list">
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"><polyline points="15 18 9 12 15 6" /></svg
      >
    </a>
    <div class="topbar-title">
      <h1>Quiz</h1>
      {#if deck}<p>{deck.title}</p>{/if}
    </div>
    {#if cards.length > 0}
      <p class="count" aria-label="{learned} of {cards.length} words learned">{learned}/{cards.length}</p>
    {/if}
  </header>

  {#if cards.length > 0}
    <div class="bar" aria-hidden="true"><span style:width="{(learned / cards.length) * 100}%"></span></div>
  {/if}

  {#if failure?.code === "not_found"}
    <main class="center">
      <p class="empty-title">This song isn't in your library</p>
    </main>
  {:else if failure}
    <main class="center"><p class="alert" role="alert">{failure.message}</p></main>
  {:else if deck === null}
    <main class="center"><p class="muted">Loading…</p></main>
  {:else if cards.length === 0}
    <main class="center">
      <div class="empty">
        <p class="empty-title">No words to study yet</p>
        <p class="muted">
          {deck.installed
            ? "None of this song's words are in the dictionary."
            : "Flashcards need the dictionary of word meanings, which isn't installed."}
        </p>
      </div>
      <a class="button" href={studyHref(songId)}>Back to the word list</a>
    </main>
  {:else if !card}
    <main class="center">
      <div class="empty">
        <p class="empty-title">All {cards.length} words learned</p>
        <p class="muted">Each one was right {goal} times in a row. Go round them all again to keep them fresh.</p>
      </div>
      <button type="button" class="button primary" onclick={() => begin(true)}>Review all again</button>
      <a class="button" href={studyHref(songId)}>Back to the word list</a>
    </main>
  {:else}
    {@const [before, word, after] = lineParts(card)}
    <main>
      <section class="card" aria-live="polite">
        <p class="word" lang="ja">{card.label}</p>
        {#if card.line}
          <p class="line" lang="ja">{before}<mark>{word}</mark>{after}</p>
        {/if}

        {#if done}
          <div class="back">
            <p class="reading" lang="ja">{card.reading}</p>
            <p class="meaning">{card.meaning}</p>
            <p class="verdict" class:right={allRight} class:wrong={!allRight}>
              {allRight ? "Right" : "Wrong"} · {Math.min(streakAfter, goal)} of {goal} in a row{streakAfter >= goal
                ? ", learned"
                : ""}
            </p>
            <div class="dots" aria-hidden="true">
              {#each Array.from({ length: goal }, (_, i) => i) as i (i)}
                <span class:on={i < streakAfter}></span>
              {/each}
            </div>
          </div>
        {/if}
      </section>

      <form class="answer" onsubmit={submit}>
        <label class="ask" class:meaning={question === "meaning" || done} for="answer">
          {#if done}
            {allRight ? "Right" : "Not this time"}
            <span>press Enter or tap Next card</span>
          {:else}
            {question === "reading" ? "Reading" : "Meaning"}
            <span>{question === "reading" ? "type it in kana or romaji" : "in English"}</span>
          {/if}
        </label>
        <div class="field" class:right={(checked && current) || (done && allRight)} class:wrong={checked && !current}>
          <input
            id="answer"
            bind:this={input}
            value={answer}
            oninput={type}
            lang={question === "reading" ? "ja" : "en"}
            autocomplete="off"
            autocapitalize="off"
            spellcheck="false"
            enterkeyhint={checked || done ? "next" : "go"}
            placeholder={question === "reading" ? "かな" : "meaning"}
          />
          <button
            type="submit"
            class="go"
            disabled={saving}
            aria-label={done ? "Next card" : checked ? "Go on" : "Check"}
          >
            <svg
              width="22"
              height="22"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"><line x1="5" y1="12" x2="19" y2="12" /><polyline points="12 5 19 12 12 19" /></svg
            >
          </button>
        </div>
        {#if checked && !current && !done}
          <div class="feedback" role="status">
            <p>
              The answer is
              <strong lang={question === "reading" ? "ja" : "en"}
                >{question === "reading"
                  ? (card.readings ?? []).join("、")
                  : (card.meanings ?? []).slice(0, 3).join(", ")}</strong
              >
            </p>
            <button type="button" class="link" onclick={countAsRight}>I was right</button>
          </div>
        {/if}
        {#if done}
          <button type="submit" class="button primary next" disabled={saving}>
            {saving ? "Saving…" : "Next card"}
          </button>
        {/if}
      </form>
    </main>
  {/if}
</div>

<style>
  .count {
    margin: 0;
    padding: 0 8px;
    font-size: 0.875rem;
    font-weight: 650;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }

  .bar {
    height: 4px;
    margin: 0 0 12px;
    border-radius: 2px;
    background: var(--surface-strong);
    overflow: hidden;
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--success);
    transition: width 0.3s;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding-bottom: max(24px, env(safe-area-inset-bottom));
  }

  .center {
    justify-content: center;
    align-items: stretch;
    gap: 12px;
    text-align: center;
  }

  .center .button {
    flex: none;
  }

  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 28px 16px 24px;
    border-radius: 20px;
    background: var(--surface);
    text-align: center;
  }

  .word {
    margin: 0;
    font-size: 2.75rem;
    font-weight: 750;
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .line {
    margin: 0;
    color: var(--muted);
    font-size: 1rem;
  }

  mark {
    background: transparent;
    color: var(--fg);
    font-weight: 700;
    text-decoration: underline dotted var(--link);
    text-decoration-thickness: 2px;
    text-underline-offset: 5px;
  }

  .back {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    margin-top: 12px;
    padding-top: 16px;
    border-top: 1px solid var(--surface-strong);
    width: 100%;
  }

  .back p {
    margin: 0;
  }

  .reading {
    font-size: 1.375rem;
    font-weight: 650;
  }

  .meaning {
    color: var(--muted);
  }

  .verdict {
    margin-top: 8px !important;
    font-weight: 650;
  }

  .verdict.right {
    color: var(--success);
  }

  .verdict.wrong {
    color: var(--danger);
  }

  .dots {
    display: flex;
    gap: 6px;
    margin-top: 4px;
  }

  .dots span {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--surface-strong);
  }

  .dots span.on {
    background: var(--success);
  }

  .answer {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .ask {
    display: flex;
    align-items: baseline;
    gap: 8px;
    font-weight: 700;
    color: var(--link);
  }

  .ask.meaning {
    color: var(--fg);
  }

  .ask span {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--muted);
  }

  .field {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 4px 4px 16px;
    border: 2px solid var(--surface-strong);
    border-radius: 16px;
    background: var(--surface);
  }

  .field:focus-within {
    border-color: var(--link);
  }

  .field.right {
    border-color: var(--success);
  }

  .field.wrong {
    border-color: var(--danger);
  }

  input {
    flex: 1;
    min-width: 0;
    min-height: 48px;
    border: 0;
    background: transparent;
    color: var(--fg);
    font-size: 1.25rem;
    outline: none;
  }

  input::placeholder {
    color: var(--dim);
  }

  .go {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border: 0;
    border-radius: 12px;
    background: var(--accent);
    color: var(--on-accent);
    cursor: pointer;
  }

  .feedback {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 4px 12px;
    color: var(--danger);
  }

  .feedback p {
    margin: 0;
  }

  .feedback strong {
    color: var(--fg);
  }

  .link {
    min-height: var(--tap);
    padding: 0 4px;
    border: 0;
    background: transparent;
    color: var(--link);
    font-weight: 600;
    text-decoration: underline;
    cursor: pointer;
  }

  .next {
    flex: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .bar span {
      transition: none;
    }
  }
</style>
