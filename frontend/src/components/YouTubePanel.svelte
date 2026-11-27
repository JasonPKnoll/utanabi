<script lang="ts">
  import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import {
    Service as YouTube,
    type Video,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/youtube";
  import { toAppError, type AppError } from "../lib/errors";
  import { clockAt, hasTimes, lineAt, lyricsTime, steer, trackTime, type Clock } from "../lib/follow";
  import {
    buffering,
    cued,
    embedURL,
    ended,
    errorNote,
    loadYouTube,
    pausedState,
    playingState,
    unstarted,
    type YouTubePlayer,
  } from "../lib/youtube";
  import TimingSheet from "./TimingSheet.svelte";
  import YouTubeVideoSheet from "./YouTubeVideoSheet.svelte";

  // The song's YouTube video, playing in the page, with its controls under it within thumb reach: the lyrics follow
  // it, read from the player every frame. Play starts it from the line you're on; a tap on a line jumps it there.
  // The video is yours for this song: found by search when the server can, or a link you paste.
  let {
    songId,
    lines,
    startMs,
    searchable,
    oncurrent,
    following = $bindable(false),
    switchTo = "",
    onswitch,
  }: {
    songId: number;
    lines: Line[];
    // Where Play starts: the line you're on.
    startMs: number;
    searchable: boolean;
    oncurrent: (index: number) => void;
    following?: boolean;
    // The other player the song can play along with, such as "YouTube", and switching to it; none when there's none.
    switchTo?: string;
    onswitch?: () => void;
  } = $props();

  let video = $state<Video | null>(null);
  let looked = $state(false);
  let failure = $state<AppError | null>(null);
  let playerError = $state("");
  let frame: HTMLIFrameElement | undefined = $state();
  let player: YouTubePlayer | null = null;
  let playerState = $state(unstarted);
  // The video's clock, read from the player and steered smoothly between its readings.
  let clock: Clock | null = null;
  let timing = $state(false);
  let choosing = $state(false);

  let synced = $derived(hasTimes(lines));
  let offset = $derived(video?.offsetMs ?? 0);
  let playing = $derived(playerState === playingState);

  async function look() {
    try {
      video = await YouTube.Video(songId);
      failure = null;
    } catch (err) {
      failure = toAppError(err);
    } finally {
      looked = true;
    }
  }
  void look();

  // A player for each video, once its frame is on the page. The frame goes with the video, and the player with it.
  $effect(() => {
    const el = frame;
    if (!el) {
      return;
    }
    let gone = false;
    playerError = "";
    loadYouTube()
      .then((api) => {
        if (gone) {
          return;
        }
        const made = new api.Player(el, {
          events: {
            onReady: () => (player = made),
            onStateChange: (event) => changed(event.data),
            onError: (event) => (playerError = errorNote(event.data)),
          },
        });
      })
      .catch(() => (playerError = "YouTube's player didn't load."));
    return () => {
      gone = true;
      player = null;
      clock = null;
      following = false;
      playerState = unstarted;
    };
  });

  function changed(state: number) {
    playerState = state;
    if (state === playingState || state === pausedState || state === buffering || state === ended) {
      following = true;
    }
    clock = { at: performance.now(), ms: (player?.getCurrentTime() ?? 0) * 1000, rate: 1, playing: playing };
  }

  // Every frame while it follows: read the player, steer the clock to it, and move the sung line with the song.
  $effect(() => {
    if (!following) {
      return;
    }
    let frameId = 0;
    let read = -1;
    let last = -2;
    const tick = () => {
      if (player) {
        const ms = player.getCurrentTime() * 1000;
        // The player says where it is a few times a second; in between, the clock counts on.
        if (ms !== read) {
          read = ms;
          clock = steer(clock, ms, playing, performance.now());
        }
        if (synced && clock) {
          const at = lineAt(lines, lyricsTime(clockAt(clock, performance.now()), offset));
          if (at >= 0 && at !== last) {
            last = at;
            oncurrent(at);
          }
        }
      }
      frameId = requestAnimationFrame(tick);
    };
    frameId = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frameId);
  });

  // Where the video is in the lyrics' own time, offset taken off, while they follow it; null otherwise.
  export function lyricsNow(): number | null {
    return following && clock ? lyricsTime(clockAt(clock, performance.now()), offset) : null;
  }

  // Jumps the video to ms, such as the start of a line tapped while the lyrics follow.
  export function seek(ms: number) {
    if (!player) {
      return;
    }
    const to = trackTime(Math.max(0, ms), offset);
    player.seekTo(to / 1000, true);
    clock = { at: performance.now(), ms: to, rate: 1, playing };
  }

  function tap() {
    if (!player) {
      return;
    }
    if (playing) {
      player.pauseVideo();
      return;
    }
    if (playerState === unstarted || playerState === cued || playerState === ended) {
      // Not started yet: from the line you're on.
      player.seekTo(trackTime(Math.max(0, startMs), offset) / 1000, true);
    }
    player.playVideo();
  }

  // Your timing while you slide it, before it's saved, and once you let go.
  function previewOffset(ms: number) {
    if (video) {
      video = { ...video, offsetMs: ms };
    }
  }

  async function setOffset(ms: number) {
    previewOffset(ms);
    try {
      await YouTube.SetOffset(songId, ms);
    } catch (err) {
      failure = toAppError(err);
    }
  }

  async function saveLink(link: string) {
    video = await YouTube.SetVideo(songId, link);
    failure = null;
  }

  async function searchAgain() {
    await YouTube.ForgetVideo(songId);
    looked = false;
    await look();
  }

  function note(): string {
    if (failure) {
      return failure.message;
    }
    if (!looked) {
      return "Looking for the song's video…";
    }
    if (!video?.id) {
      return searchable ? "No video found. Add a YouTube link." : "Add a YouTube link to play the song's video.";
    }
    if (playerError) {
      return playerError;
    }
    if (following) {
      if (!synced) {
        return `${playing ? "Playing" : "Paused"}. These lyrics aren't synced, so they won't follow.`;
      }
      return playing ? "Playing on YouTube, the lyrics follow" : "Paused";
    }
    return "Play the video";
  }
</script>

<div class="youtube">
  {#if video?.id}
    {#key video.id}
      <!-- The page sends no referrer, but YouTube's player won't play without knowing the page it's on. -->
      <iframe
        bind:this={frame}
        class="video"
        title={video.title || "The song's video on YouTube"}
        src={embedURL(video.id, window.location.origin)}
        allow="autoplay; encrypted-media; picture-in-picture"
        referrerpolicy="strict-origin-when-cross-origin"
        allowfullscreen
      ></iframe>
    {/key}
  {/if}
  <div class="controls">
    <button
      type="button"
      class="play"
      disabled={!video?.id || !!playerError}
      aria-label={playing ? "Pause the video" : "Play the video"}
      onclick={tap}
    >
      {#if playing}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"
          ><rect x="6" y="5" width="4" height="14" rx="1" /><rect x="14" y="5" width="4" height="14" rx="1" /></svg
        >
      {:else}
        <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"
          ><path d="M8 5.5v13a1 1 0 0 0 1.5.86l10.5-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5z" /></svg
        >
      {/if}
    </button>
    <p class="note" class:alert={failure !== null} role="status">{note()}</p>
    {#if following && synced}
      <button type="button" class="icon-button" aria-label="Lyrics timing" onclick={() => (timing = true)}>
        <svg
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"><circle cx="12" cy="13" r="8" /><path d="M12 9v4l2.5 2.5M10 2h4M12 2v3" /></svg
        >
      </button>
    {/if}
    <button type="button" class="icon-button" aria-label="Choose the video" onclick={() => (choosing = true)}>
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
        ><path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7" /><path
          d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"
        /></svg
      >
    </button>
    {#if switchTo && onswitch}
      <button type="button" class="switch" aria-label={`Play along with ${switchTo} instead`} onclick={onswitch}
        >{switchTo}</button
      >
    {/if}
  </div>
</div>

{#if timing && video}
  <TimingSheet
    offsetMs={video.offsetMs}
    offsetSet={video.offsetMs !== 0}
    {lines}
    now={() => (clock ? clockAt(clock, performance.now()) : 0)}
    onpreview={previewOffset}
    onchange={setOffset}
    onreset={() => setOffset(0)}
    onclose={() => (timing = false)}
  />
{/if}

{#if choosing}
  <YouTubeVideoSheet {video} {searchable} onsave={saveLink} onsearch={searchAgain} onclose={() => (choosing = false)} />
{/if}

<style>
  .youtube {
    position: sticky;
    bottom: 0;
    margin-top: auto;
    padding: 12px 0 max(16px, env(safe-area-inset-bottom));
    background: linear-gradient(to bottom, transparent, var(--bg) 24px);
  }

  /* YouTube asks for an embedded player of at least 200 pixels each way. */
  .video {
    display: block;
    width: 100%;
    min-width: 200px;
    height: 200px;
    border: 0;
    border-radius: 14px;
    background: var(--surface);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 10px;
  }

  .play {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 52px;
    height: 52px;
    border: 0;
    border-radius: 50%;
    background: var(--accent);
    color: var(--on-accent);
    cursor: pointer;
  }

  .play:disabled {
    opacity: 0.6;
  }

  .note {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 0.875rem;
    color: var(--muted);
  }

  .note.alert {
    padding: 0;
    background: transparent;
  }

  /* Switches the song to the other player, when there is one. */
  .switch {
    flex: none;
    min-height: var(--tap);
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 22px;
    background: transparent;
    color: var(--fg);
    font: inherit;
    font-size: 0.8125rem;
    font-weight: 600;
    cursor: pointer;
  }
</style>
