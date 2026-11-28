<script lang="ts">
  import { untrack } from "svelte";

  import {
    Service as Spotify,
    type Device,
    type Playback,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
  import type { Line } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError, type AppError } from "../lib/errors";
  import {
    answeredAt,
    caughtUp,
    clockAt,
    hasTimes,
    lineAt,
    lyricsTime,
    steer,
    trackLink,
    trackTime,
    type Clock,
    type Move,
  } from "../lib/follow";
  import { chooseDevice, choiceOf, loadDevice, saveDevice } from "../lib/devices";
  import { formatDuration } from "../lib/sources";
  import { activateWebPlayer, startWebPlayer, webPlayer, webPlayerName } from "../lib/webPlayer.svelte";
  import LyricsForTrack from "./LyricsForTrack.svelte";
  import SpotifyDeviceSheet from "./SpotifyDeviceSheet.svelte";
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
    switchTo = "",
    onswitch,
    lyricsMs = 0,
    query = "",
  }: {
    songId: number;
    lines: Line[];
    // Where Play starts: the line you're on.
    startMs: number;
    oncurrent: (index: number) => void;
    following?: boolean;
    // The other player the song can play along with, such as "YouTube", and switching to it; none when there's none.
    switchTo?: string;
    onswitch?: () => void;
    // How long the lyrics are timed for, and what to search for lyrics timed for the track instead.
    lyricsMs?: number;
    query?: string;
  } = $props();

  let connected = $state(false);
  let playback = $state<Playback | null>(null);
  // The song's clock, set from Spotify's answers and steered smoothly between them.
  let clock: Clock | null = null;
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

  // Where the song is on the track now, by the clock.
  function trackNow(): number {
    return clock ? clockAt(clock, performance.now()) : 0;
  }

  // Whether the connection lets this page play: one made before the app asked for streaming has to be made again.
  let canPlayHere = $state(false);

  Spotify.Status()
    .then((s) => {
      connected = s.enabled && s.connected;
      canPlayHere = s.canPlayHere;
    })
    .catch(() => (connected = false));

  // The last move you made, until Spotify's answers show it.
  let move: Move | null = null;

  async function ask() {
    try {
      const askedAt = performance.now();
      let p = await Spotify.NowPlaying(songId);
      if (unsaved !== null) {
        p = { ...p, offsetMs: unsaved, offsetSet: true, timedForMs: 0 };
      }
      const now = performance.now();
      const at = answeredAt(p.progressMs, p.playing, askedAt, now);
      failure = null;
      if (move && playback && !caughtUp(move, { ...p, progressMs: at }, now)) {
        // Spotify hasn't made the move yet: the lyrics wait where it will put the song, and take the rest.
        playback = { ...p, progressMs: playback.progressMs, playing: playback.playing, matches: true, active: true };
        return;
      }
      // Still this song, a little off: steer towards Spotify. Just after a move, or anything else, sets the clock
      // outright, from where Spotify is.
      const same = !move && following && p.active && p.matches && playback?.matches;
      clock = steer(same ? clock : null, at, p.playing, now);
      move = null;
      playback = p;
      following = p.active && p.matches;
    } catch (err) {
      failure = toAppError(err);
    }
  }

  // Shows a move at once, before Spotify does: the lines follow from where it puts the song, not from where Spotify
  // last said it was.
  function moved(trackMs: number, playingNow: boolean) {
    const now = performance.now();
    move = { trackMs, playing: playingNow, at: now };
    if (playback) {
      playback = { ...playback, progressMs: trackMs, playing: playingNow, active: true, matches: true };
      // The lyrics go there at once, but hold still: Spotify takes a moment to jump, and they'd run ahead of the
      // song. They count on once Spotify says it's there.
      clock = { at: now, ms: trackMs, rate: 1, playing: false };
      following = true;
    }
    soon?.();
  }

  // Ask while the page is on screen: every 2 seconds while this song plays, every 5 otherwise. Only connecting
  // starts or stops it: the answers it reads are untracked, so each one doesn't start it over.
  $effect(() => {
    if (!connected) {
      return;
    }
    // This page as a player too, so a song can play here when the Spotify app isn't open.
    if (canPlayHere) {
      startWebPlayer(() => Spotify.BrowserToken().then((t) => t.accessToken));
    }
    return untrack(() => poll());
  });

  // Asks again soon, after a move, rather than when the next ask was due; set while asking runs.
  let soon: (() => void) | null = null;

  function poll(): () => void {
    void ask();
    let timer = 0;
    const schedule = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(
        async () => {
          if (document.visibilityState === "visible") {
            await ask();
          }
          schedule();
        },
        // Every half second while waiting for Spotify to make a move, so the lyrics don't wait long.
        move ? 500 : following && playback?.playing ? 2000 : 5000,
      );
    };
    schedule();
    soon = schedule;
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
      soon = null;
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

  // Where Spotify plays: the device you chose, remembered on this device, and while choosing, the sheet. On a phone,
  // with no choice made, your phone comes first, even when Spotify last played on your computer.
  let deviceChoice = $state(loadDevice());
  let choosingDevice = $state(false);
  const onPhone = window.matchMedia?.("(pointer: coarse)").matches ?? false;

  // The device to play on now, or null when Spotify can't say (it plays where it likes).
  async function where(): Promise<Device | null> {
    try {
      return chooseDevice((await Spotify.Devices()) ?? [], deviceChoice, onPhone, webPlayer.deviceId ?? "");
    } catch {
      return null;
    }
  }

  function chooseOn(device: Device) {
    deviceChoice = choiceOf(device, webPlayer.deviceId ?? "");
    // When storage refuses, the choice still holds until the screen closes.
    saveDevice(deviceChoice);
    if (device.id === webPlayer.deviceId) {
      activateWebPlayer();
    }
    if (following) {
      void play(device.id, trackNow());
    }
  }

  // Carries on with the song: on the device it's paused on, or, when you'd rather it played elsewhere (your phone,
  // when you're on your phone), there, from where it's paused.
  async function resume() {
    const device = await where();
    const there = device && (device.id === webPlayer.deviceId ? webPlayerName : device.name);
    if (device && there !== playback?.device) {
      await play(device.id, trackNow());
      return;
    }
    moved(trackNow(), true);
    await control(() => Spotify.Resume());
  }

  // Plays the song from fromMs on the track, or the line you're on, on device, or with none given, the device you'd
  // play on.
  async function play(device = "", fromMs: number | null = null) {
    busy = true;
    failure = null;
    try {
      if (!device) {
        device = (await where())?.id ?? "";
      }
      const from = fromMs ?? trackTime(Math.max(0, startMs), offset);
      const result = await Spotify.Play(songId, from, device);
      if (result.started) {
        moved(from, true);
        await ask();
      } else {
        if (device && device === webPlayer.deviceId) {
          // The page's player couldn't play, and nothing else could either: don't try it again on this visit.
          webPlayer.status = "unavailable";
        }
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
    // The page's player may make sound only from a tap, on a phone; this is the tap.
    if (webPlayer.status === "ready") {
      activateWebPlayer();
    }
    if (playing) {
      moved(trackNow(), false);
      void control(() => Spotify.Pause());
    } else if (following) {
      void resume();
    } else if (webPlayer.status === "ready" && !playback?.active && !deviceChoice) {
      // Nothing playing anywhere, and no device chosen: play here, in the page, rather than asking you to open the
      // Spotify app.
      void play(webPlayer.deviceId);
    } else if (playback && !playback.ready && webPlayer.status !== "ready" && trackLink(playback.trackUri)) {
      // No Spotify open anywhere, so the API can't reach one: ask you to open it.
      opening = trackLink(playback.trackUri);
    } else {
      void play();
    }
  }

  // Your timing for this song while you slide it, until it's saved: the lyrics move with it, and Spotify's answers
  // in the meantime, which still carry the timing saved before, don't move it back.
  let unsaved: number | null = null;

  function previewOffset(ms: number) {
    unsaved = ms;
    if (playback) {
      playback = { ...playback, offsetMs: ms, offsetSet: true, timedForMs: 0 };
    }
  }

  // Your timing for this song: shown at once, and saved for next time.
  async function setOffset(ms: number) {
    previewOffset(ms);
    try {
      await Spotify.SetOffset(songId, ms);
    } catch (err) {
      failure = toAppError(err);
    } finally {
      if (unsaved === ms) {
        unsaved = null;
      }
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

  // Where the song is in the lyrics' own time, offset taken off, while they follow it; null otherwise.
  export function lyricsNow(): number | null {
    return following && playback ? lyricsTime(trackNow(), offset) : null;
  }

  // Jumps the song to ms, such as the start of a line tapped while the lyrics follow.
  export function seek(ms: number) {
    const to = trackTime(Math.max(0, ms), offset);
    moved(to, playback?.playing ?? true);
    void control(() => Spotify.Seek(to));
  }

  async function control(fn: () => Promise<void>) {
    busy = true;
    failure = null;
    try {
      await fn();
      await ask();
    } catch (err) {
      // The move didn't happen: Spotify's next answer is where the song is.
      move = null;
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
      const where = playback.device === webPlayerName ? " here" : playback.device ? ` on ${playback.device}` : "";
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
    if (!canPlayHere || webPlayer.status === "reconnect") {
      return "Play on Spotify. Reconnect it in Settings to play here too.";
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
    <button
      type="button"
      class="icon-button"
      aria-label="Choose where Spotify plays"
      onclick={() => (choosingDevice = true)}
    >
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
        ><rect x="5" y="2" width="14" height="20" rx="2.5" /><circle cx="12" cy="14" r="3.5" /><path
          d="M12 6.5h.01"
        /></svg
      >
    </button>
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
    {#if switchTo && onswitch}
      <button type="button" class="switch" aria-label={`Play along with ${switchTo} instead`} onclick={onswitch}
        >{switchTo}</button
      >
    {/if}
  </div>
{/if}

{#if timing && playback}
  <TimingSheet
    offsetMs={playback.offsetMs}
    offsetSet={playback.offsetSet}
    {lines}
    now={trackNow}
    onpreview={previewOffset}
    onchange={setOffset}
    onreset={resetOffset}
    onclose={() => (timing = false)}
  >
    {#snippet more()}
      <LyricsForTrack {songId} {lyricsMs} trackMs={playback?.trackMs ?? 0} {query} />
    {/snippet}
  </TimingSheet>
{/if}

{#if choosingDevice}
  <SpotifyDeviceSheet
    hereId={webPlayer.deviceId ?? ""}
    chosen={deviceChoice}
    playingOn={playback?.active ? playback.device : ""}
    paused={!playback?.playing}
    onchoose={chooseOn}
    onclose={() => (choosingDevice = false)}
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
