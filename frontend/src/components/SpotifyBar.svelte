<script lang="ts">
  import { untrack } from "svelte";

  import {
    Service as Spotify,
    type Playback,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
  import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import { hasTimes, lineAt, lyricsTime, positionAt, trackLink, trackTime } from "../lib/follow";
  import { formatDuration } from "../lib/sources";
  import SpotifyOpenSheet from "./SpotifyOpenSheet.svelte";
  import TimingSheet from "./TimingSheet.svelte";

  // The song's Spotify controls, at the bottom within thumb reach, once you've connected Spotify in Settings. Play
  // starts the song on your Spotify, from the line you're on, and the lyrics follow it: the app asks Spotify how far
  // in it is every couple of seconds and counts on in between. following says whether they're following now, so a
  // tap on a line can jump the song there. When no Spotify is open to play on, Play asks you to open it, in a sheet
  // of the app's own; coming back to this page then starts the song.
  let {
    songId,
    lines,
    startMs,
    oncurrent,
    following = $bindable(false),
  }: {
    songId: number;
    lines: Line[];
    // Where Play starts: the line you're on.
    startMs: number;
    oncurrent: (index: number) => void;
    following?: boolean;
  } = $props();

  let connected = $state(false);
  let playback = $state<Playback | null>(null);
  let askedAt = 0;
  let busy = $state(false);
  let failure = $state<AppError | null>(null);
  // Play was asked for while no Spotify was open: start it once this page is back on screen.
  let waiting = $state(false);
  // The track's web link while the sheet asking you to open Spotify is up.
  let opening = $state("");
  let timing = $state(false);

  let synced = $derived(hasTimes(lines));
  let playing = $derived(following && (playback?.playing ?? false));
  // How far the lyrics are shifted onto the track; lines, Play, and jumps all go through it.
  let offset = $derived(playback?.offsetMs ?? 0);

  // Where the song is on the track now, counting on from Spotify's last answer.
  function trackNow(): number {
    return playback ? positionAt(playback.progressMs, playback.playing, askedAt, performance.now()) : 0;
  }

  Spotify.Status()
    .then((s) => (connected = s.enabled && s.connected))
    .catch(() => (connected = false));

  async function ask() {
    try {
      const p = await Spotify.NowPlaying(songId);
      playback = p;
      askedAt = performance.now();
      following = p.active && p.matches;
      failure = null;
    } catch (err) {
      failure = toAppError(err);
    }
  }

  // Ask while the page is on screen: every 2 seconds while this song plays, every 5 otherwise. Only connecting
  // starts or stops it: the answers it reads are untracked, so each one doesn't start it over.
  $effect(() => {
    if (!connected) {
      return;
    }
    return untrack(() => poll());
  });

  function poll(): () => void {
    void ask();
    let timer = 0;
    const schedule = () => {
      timer = window.setTimeout(
        async () => {
          if (document.visibilityState === "visible") {
            await ask();
          }
          schedule();
        },
        following && playback?.playing ? 2000 : 5000,
      );
    };
    schedule();
    const back = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      if (waiting) {
        waiting = false;
        void play();
      } else {
        void ask();
      }
    };
    document.addEventListener("visibilitychange", back);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", back);
    };
  }

  // Move the sung line with the song, a few times a second.
  $effect(() => {
    if (!connected || !synced) {
      return;
    }
    let last = -2;
    const timer = window.setInterval(() => {
      if (!following || !playback) {
        return;
      }
      const at = lineAt(lines, lyricsTime(trackNow(), offset));
      if (at >= 0 && at !== last) {
        last = at;
        oncurrent(at);
      }
    }, 200);
    return () => window.clearInterval(timer);
  });

  async function play() {
    busy = true;
    failure = null;
    try {
      const result = await Spotify.Play(songId, trackTime(Math.max(0, startMs), offset));
      if (result.started) {
        await ask();
      } else {
        // No Spotify open to play on: ask you to open it.
        opening = trackLink(result.trackUri);
      }
    } catch (err) {
      failure = toAppError(err);
    } finally {
      busy = false;
    }
  }

  function tap() {
    if (playing) {
      void control(() => Spotify.Pause());
    } else if (following) {
      void control(() => Spotify.Resume());
    } else if (playback && !playback.ready && trackLink(playback.trackUri)) {
      // No Spotify open anywhere, so the API can't reach one: ask you to open it.
      opening = trackLink(playback.trackUri);
    } else {
      void play();
    }
  }

  // Your timing for this song: shown at once, and saved for next time.
  async function setOffset(ms: number) {
    if (playback) {
      playback = { ...playback, offsetMs: ms, offsetSet: true, timedForMs: 0 };
    }
    try {
      await Spotify.SetOffset(songId, ms);
    } catch (err) {
      failure = toAppError(err);
    }
  }

  async function resetOffset() {
    try {
      await Spotify.ResetOffset(songId);
      await ask();
    } catch (err) {
      failure = toAppError(err);
    }
  }

  // Jumps the song to ms, such as the start of a line tapped while the lyrics follow.
  export function seek(ms: number) {
    void control(() => Spotify.Seek(trackTime(Math.max(0, ms), offset)));
  }

  async function control(fn: () => Promise<void>) {
    busy = true;
    failure = null;
    try {
      await fn();
      await ask();
    } catch (err) {
      failure = toAppError(err);
    } finally {
      busy = false;
    }
  }

  function note(): string {
    if (failure) {
      return failure.message;
    }
    if (!playback) {
      return "Checking your Spotify…";
    }
    if (!playback.trackUri && !following) {
      return "This song isn't on Spotify";
    }
    if (following) {
      const where = playback.device ? ` on ${playback.device}` : "";
      if (!synced) {
        return `${playback.playing ? "Playing" : "Paused"}${where}. These lyrics aren't synced, so they won't follow.`;
      }
      if (playback.timedForMs) {
        // Lyrics from a video or a live recording: following would put the wrong line up.
        return `${playback.playing ? "Playing" : "Paused"}${where}. These lyrics are timed for a ${formatDuration(playback.timedForMs)} version: set the timing to line them up.`;
      }
      return playback.playing ? `Playing${where}, the lyrics follow` : `Paused${where}`;
    }
    if (waiting) {
      return "Open Spotify, then come back here";
    }
    if (playback.active && playback.playing) {
      return "Something else is playing on Spotify";
    }
    return "Play on Spotify";
  }
</script>

{#if connected}
  <div class="actions spotify">
    <button
      type="button"
      class="play"
      disabled={busy}
      aria-label={playing ? "Pause on Spotify" : "Play on Spotify"}
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
  </div>
{/if}

{#if timing && playback}
  <TimingSheet
    offsetMs={playback.offsetMs}
    offsetSet={playback.offsetSet}
    {lines}
    now={trackNow}
    onchange={setOffset}
    onreset={resetOffset}
    onclose={() => (timing = false)}
  />
{/if}

{#if opening}
  <SpotifyOpenSheet link={opening} onopen={() => (waiting = true)} onclose={() => (opening = "")} />
{/if}

<style>
  .spotify {
    align-items: center;
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
</style>
