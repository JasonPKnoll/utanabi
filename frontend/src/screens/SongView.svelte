<script lang="ts">
  import {
    Service as Songs,
    type RubyPart,
    type Song,
    type Word,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import DisplaySheet from "../components/DisplaySheet.svelte";
  import Interlinear from "../components/Interlinear.svelte";
  import SpotifyBar from "../components/SpotifyBar.svelte";
  import YouTubePanel from "../components/YouTubePanel.svelte";
  import { Service as Spotify } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
  import { Service as YouTube } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/youtube";
  import { loadPlayer, playerFor, savePlayer, type Player } from "../lib/players";
  import Firework from "../components/Firework.svelte";
  import WordSheet from "../components/WordSheet.svelte";
  import { toAppError, type AppError } from "../lib/errors";
  import { startsStanza } from "../lib/lyrics";
  import { meaningsCache } from "../lib/meaningsCache";
  import { placeholderLines } from "../lib/placeholders";
  import { hasRomaji, loadReadingAids, saveReadingAids, type ReadingAids } from "../lib/readingAids";
  import { aboutHref, homeHref, studyHref } from "../lib/route";
  import { sourceLabel } from "../lib/sources";
  import { lineSweep, songPace } from "../lib/sweep";
  import { searchFor } from "../lib/versions";

  let { id }: { id: number } = $props();

  let song = $state<Song | null>(null);
  let failure = $state<AppError | null>(null);
  let current = $state(0);
  let deleting = $state(false);
  let aids = $state(loadReadingAids());
  let displayOpen = $state(false);
  let open = $state<{ word: Word; ruby: RubyPart[]; line: number } | null>(null);
  // While the lyrics follow the player, Spotify or YouTube, the sung line moves with the song, and a tapped line jumps
  // the song there.
  let following = $state(false);
  let bar: SpotifyBar | YouTubePanel | undefined = $state();

  // The players there are to follow: Spotify once you've connected it, and YouTube on a server. Your choice between
  // them is remembered on this device.
  let available = $state<Player[]>([]);
  let youtubeSearch = $state(false);
  let chosen = $state<Player>(loadPlayer());
  let player = $derived(playerFor(chosen, available));

  Promise.all([Spotify.Status().catch(() => null), YouTube.Status().catch(() => null)]).then(([spotify, youtube]) => {
    const there: Player[] = [];
    if (spotify?.enabled && spotify.connected) {
      there.push("spotify");
    }
    if (youtube?.enabled) {
      there.push("youtube");
      youtubeSearch = youtube.searchable;
    }
    available = there;
  });

  function choosePlayer(p: Player) {
    chosen = p;
    following = false;
    // When storage refuses, the choice still holds until the screen closes.
    savePlayer(p);
  }

  let lines = $derived(song?.lines ?? []);
  // While the song loads, stand-in lines go through the same markup, as a skeleton in the song's shape.
  let loading = $derived(song === null && failure === null);
  let shown = $derived(loading ? placeholderLines() : lines);
  // The lyric time, read from the Spotify bar every frame while the lyrics follow it, and how far through each piece
  // of the current line the singing is, for the sweep across it.
  let clock = $state<number | null>(null);
  // How fast the song is sung, measured once from its lines, so each line's sweep ends when its words are sung, not
  // when the next line starts after a pause.
  let pace = $derived(songPace(lines));
  let sung = $derived(lineSweep(lines, current, clock, pace));
  $effect(() => {
    if (!following) {
      clock = null;
      return;
    }
    let frame = 0;
    const tick = () => {
      clock = bar?.lyricsNow() ?? null;
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  });
  // JMdict's licence asks for a credit wherever its meanings show, so the footnote appears only when they do.
  let hasDefinitions = $derived(lines.some((l) => (l.words ?? []).some((w) => w.entryId > 0)));
  let source = $derived.by(() => {
    if (!song || song.source === "user") {
      return "";
    }
    return sourceLabel(song.source) + (lines.some((l) => l.startMs !== null) ? ", synced" : "");
  });

  function toggle(aid: keyof ReadingAids) {
    aids[aid] = !aids[aid];
    // When storage refuses, the choice still holds until the screen closes, which is all that can be done.
    saveReadingAids(aids);
  }

  $effect(() => {
    Songs.Get(id)
      .then((found) => {
        song = found;
        // Fetch every word's meanings now, in one call, so tapping a word opens its sheet at once.
        void meaningsCache.preload((found.lines ?? []).flatMap((line) => line.words ?? []));
      })
      .catch((err: unknown) => (failure = toAppError(err)));
  });

  // Picks a line: a tap, or the song moving on while the lyrics follow Spotify.
  function select(i: number) {
    current = i;
    const startMs = lines[i]?.startMs;
    if (following && startMs !== null && startMs !== undefined) {
      bar?.seek(startMs);
    }
  }

  function follow(i: number) {
    if (i === current) {
      return;
    }
    current = i;
    const still = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
    document.getElementById(`line-${i}`)?.scrollIntoView({ block: "center", behavior: still ? "auto" : "smooth" });
  }

  // Saves how a word is sung, then shows the song and the open word as they are now. A failure is the sheet's to show.
  async function setSung(line: number, firstToken: number, reading: string) {
    const updated = await Songs.SetSungReading({ songId: id, line, firstToken, reading });
    song = updated;
    const shown = updated.lines?.find((l) => l.position === line);
    const index = shown?.words?.findIndex((w) => w.firstToken === firstToken) ?? -1;
    const piece = shown?.pieces?.find((p) => p.word === index);
    const word = shown?.words?.[index];
    if (word && piece) {
      open = { word, ruby: piece.ruby ?? [], line };
    }
  }

  async function remove() {
    deleting = true;
    try {
      await Songs.Delete(id);
      window.location.replace(homeHref);
    } catch (err) {
      failure = toAppError(err);
      deleting = false;
      displayOpen = false;
    }
  }
</script>

<!-- prettier-ignore -->
{#snippet rubyText(parts: RubyPart[])}{#each parts as part, k (k)}{#if aids.furigana && part.reading}<ruby>{part.text}<rt>{part.reading}</rt></ruby>{:else}{part.text}{/if}{/each}{/snippet}

<div class="screen">
  <header class="topbar">
    <a class="icon-button" href={homeHref} aria-label="Back to your songs">
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
      {#if song}
        <h1>{song.title}</h1>
        <p>{song.artist || "Unknown artist"}</p>
      {:else if loading}
        <h1>
          <span class="skeleton-wait" aria-hidden="true"><span class="ph">A song title</span></span>
          <span class="visually-hidden">Loading the song</span>
        </h1>
        <p class="skeleton-wait" aria-hidden="true"><span class="ph">Artist name</span></p>
      {/if}
    </div>
    {#if song}
      <a class="icon-button" href={studyHref(id)} aria-label="Study this song's words">
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
          ><rect x="3" y="6" width="14" height="14" rx="2.5" /><path d="M7 3h11.5A2.5 2.5 0 0 1 21 5.5V17" /></svg
        >
      </a>
      <button
        type="button"
        class="icon-button display"
        aria-label="Display options"
        onclick={() => (displayOpen = true)}
      >
        <span aria-hidden="true">Aa</span>
      </button>
    {/if}
  </header>

  {#if failure?.code === "not_found"}
    <main class="center">
      <p class="empty-title">This song isn't in your library</p>
      <a class="button primary" href={homeHref}>Back to your songs</a>
    </main>
  {:else if failure}
    <main class="center">
      <p class="alert" role="alert">{failure.message}</p>
    </main>
  {:else}
    <main>
      {#if loading}
        <p class="visually-hidden" role="status">Opening the song</p>
        <!-- An older song is built the first time it's opened. When the open runs that long, the firework takes over
             from the skeleton. -->
        <div class="opening" aria-hidden="true">
          <Firework size={160} />
          <p class="muted">Building this song…</p>
        </div>
      {/if}
      <ol
        class="lyrics"
        class:furigana={aids.furigana}
        class:words={aids.words}
        class:skeleton-wait={loading}
        class:placeholder={loading}
        inert={loading}
        aria-hidden={loading ? "true" : undefined}
        lang="ja"
      >
        {#each shown as line, i (line.position)}
          <li id={loading ? undefined : `line-${i}`} class:stanza-start={startsStanza(shown, i)}>
            {#if aids.words && (line.words ?? []).length > 0}
              {#if i === current}
                <div class="line current card" role="group" aria-label={line.text} aria-current="true">
                  <Interlinear
                    {line}
                    furigana={aids.furigana}
                    placeholder={loading}
                    sweep={loading ? null : sung}
                    onword={(word, ruby) => (open = { word, ruby, line: line.position })}
                  />
                  {#if aids.romaji && hasRomaji(line)}
                    <span class="romaji" lang="ja-Latn"><span class:ph={loading}>{line.romaji}</span></span>
                  {/if}
                </div>
              {:else}
                <button
                  type="button"
                  class="line card"
                  aria-current={false}
                  aria-label={line.text}
                  onclick={() => select(i)}
                >
                  <Interlinear {line} furigana={aids.furigana} placeholder={loading} />
                  {#if aids.romaji && hasRomaji(line)}
                    <span class="romaji" lang="ja-Latn"><span class:ph={loading}>{line.romaji}</span></span>
                  {/if}
                </button>
              {/if}
            {:else if i === current && (line.words ?? []).length > 0}
              <div class="line current" role="group" aria-label={line.text} aria-current="true">
                <!-- prettier-ignore -->
                <span class="sung"><span class:ph={loading}>{#each line.pieces ?? [] as piece, j (j)}{@const word = piece.word >= 0 ? line.words?.[piece.word] : undefined}{@const p = loading ? undefined : sung?.[j]}{#if word}<button type="button" class="word" aria-label={word.text} onclick={() => (open = { word, ruby: piece.ruby ?? [], line: line.position })}><span class:sweep={p !== undefined} style:--sung={p === undefined ? undefined : `${p * 100}%`}>{@render rubyText(piece.ruby ?? [])}</span></button>{:else}<span class:sweep={p !== undefined} style:--sung={p === undefined ? undefined : `${p * 100}%`}>{@render rubyText(piece.ruby ?? [])}</span>{/if}{/each}</span></span>
                {#if aids.romaji && hasRomaji(line)}
                  <span class="romaji" lang="ja-Latn"><span class:ph={loading}>{line.romaji}</span></span>
                {/if}
              </div>
            {:else}
              <button
                type="button"
                class="line"
                class:current={i === current}
                aria-current={i === current}
                aria-label={line.text}
                onclick={() => select(i)}
              >
                <!-- prettier-ignore -->
                <span class="sung"><span class:ph={loading}>{@render rubyText(line.ruby ?? [{ text: line.text, reading: "" }])}</span></span>
                {#if aids.romaji && hasRomaji(line)}
                  <span class="romaji" lang="ja-Latn"><span class:ph={loading}>{line.romaji}</span></span>
                {/if}
              </button>
            {/if}
          </li>
        {/each}
      </ol>
      {#if aids.words && hasDefinitions}
        <p class="footnote">Definitions from JMdict · <a href={aboutHref}>Sources</a></p>
      {/if}
    </main>

    {#if song && player === "spotify"}
      <SpotifyBar
        bind:this={bar}
        bind:following
        songId={id}
        {lines}
        startMs={lines[current]?.startMs ?? 0}
        oncurrent={follow}
        switchTo={available.includes("youtube") ? "YouTube" : ""}
        lyricsMs={song.durationMs ?? 0}
        query={searchFor(song.title, song.artist)}
        onswitch={() => choosePlayer("youtube")}
      />
    {:else if song && player === "youtube"}
      <YouTubePanel
        bind:this={bar}
        bind:following
        songId={id}
        {lines}
        startMs={lines[current]?.startMs ?? 0}
        searchable={youtubeSearch}
        query={searchFor(song.title, song.artist)}
        oncurrent={follow}
        switchTo={available.includes("spotify") ? "Spotify" : ""}
        onswitch={() => choosePlayer("spotify")}
      />
    {/if}

    {#if open}
      <WordSheet
        word={open.word}
        ruby={open.ruby}
        onclose={() => (open = null)}
        onsung={(reading) => setSung(open?.line ?? 0, open?.word.firstToken ?? 0, reading)}
      />
    {/if}

    {#if displayOpen}
      <DisplaySheet
        {aids}
        {source}
        {deleting}
        players={available}
        {player}
        onplayer={choosePlayer}
        ontoggle={toggle}
        ondelete={remove}
        onclose={() => (displayOpen = false)}
      />
    {/if}
  {/if}
</div>

<style>
  /* Over the whole screen, as the song-building overlay is: the skeleton under it fades in, and that puts it in a layer
     of its own, which an overlay without a z-index sits beneath. */
  .opening {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: var(--bg);
    animation: appear 0.3s ease-out 1.25s both;
  }

  @keyframes appear {
    from {
      opacity: 0;
    }
  }

  .display {
    font-size: 1.0625rem;
    font-weight: 700;
    letter-spacing: -0.02em;
  }

  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 8px 0 max(32px, env(safe-area-inset-bottom));
  }

  .center {
    justify-content: center;
    align-items: center;
    gap: 16px;
    text-align: center;
  }

  .center .button {
    flex: none;
  }

  .lyrics {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .stanza-start {
    margin-top: 24px;
  }

  /* border-box because the current line is a <div> and the others are <button>s, which are border-box already:
     without it the current line's padding came on top of its width and pushed it past the right edge. */
  .line {
    box-sizing: border-box;
    display: block;
    width: 100%;
    min-height: var(--tap);
    padding: 6px 8px;
    border: 0;
    border-radius: 16px;
    background: transparent;
    color: var(--dim);
    font-size: 1.125rem;
    font-weight: 500;
    line-height: 1.7;
    text-align: center;
    cursor: pointer;
    transition:
      color 0.15s,
      font-size 0.15s;
  }

  .line:hover {
    color: var(--muted);
  }

  .line.current {
    color: var(--fg);
    font-size: 1.5rem;
    font-weight: 700;
  }

  .sung {
    display: block;
  }

  /* Furigana sits above the line inside its line height, so the rows need room for it. */
  .furigana .line {
    line-height: 2.2;
  }

  .line :global(rt) {
    font-size: max(0.5em, 10px);
    font-weight: 400;
    line-height: 1;
    color: inherit;
  }

  .line.current :global(rt) {
    color: var(--muted);
  }

  .romaji {
    display: block;
    margin-bottom: 4px;
    color: var(--dim);
    font-size: 0.9375rem;
    font-weight: 400;
    line-height: 1.4;
  }

  .line.current .romaji {
    color: var(--muted);
  }

  /* With Words on, each line is a soft card, so it's clear where one line and its English end and the next begin.
     The current line's card is brighter. */
  .words .card {
    margin: 4px 0;
    padding: 10px 6px;
    background: color-mix(in srgb, var(--surface) 55%, transparent);
  }

  .words .card.current {
    background: var(--surface);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--link) 45%, transparent);
  }

  .words .card .romaji {
    margin-top: 6px;
  }

  /* The words of the current line. Each is a real button, at least 44px each way, marked with an amber underline so
     it reads as tappable without hover. */
  .word {
    display: inline-block;
    min-width: var(--tap);
    min-height: var(--tap);
    margin: 0 1px;
    padding: 0 2px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    font: inherit;
    line-height: inherit;
    text-decoration: underline dotted var(--link);
    text-decoration-thickness: 2px;
    text-underline-offset: 6px;
    cursor: pointer;
  }

  .word:hover {
    background: var(--surface);
  }

  /* A skeleton has the current line's shape, but nothing in it to tap. */
  .placeholder .word,
  .placeholder :global(.word .ja) {
    text-decoration: none;
  }

  .placeholder :global(.current.card) {
    box-shadow: none;
    outline: none;
    border-color: transparent;
  }

  @media (prefers-reduced-motion: reduce) {
    .line {
      transition: none;
    }
  }

  @media (min-width: 640px) {
    .line {
      font-size: 1.25rem;
    }

    .line.current {
      font-size: 1.75rem;
    }
  }
</style>
