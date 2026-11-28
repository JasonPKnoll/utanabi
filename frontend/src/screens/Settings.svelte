<script lang="ts">
  import {
    Service as Spotify,
    type Status as SpotifyStatus,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
  import {
    Service as YouTube,
    type Status as YouTubeStatus,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/youtube";
  import ReadingAidSwitches from "../components/ReadingAidSwitches.svelte";
  import ThemePicker from "../components/ThemePicker.svelte";
  import { account, signedOut } from "../lib/account.svelte";
  import { addPasskey, rememberAccountHere, signOut } from "../lib/auth";
  import { toAppError } from "../lib/errors";
  import { loadReadingAids, saveReadingAids, type ReadingAids } from "../lib/readingAids";
  import { aboutHref, homeHref, invitesHref, toolsHref } from "../lib/route";
  import { Tools } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";

  // The choices you make about the app, first, where they're quick to reach: the reading aids, the theme, and your
  // account. The version and the credits are a tap away on About, so they never push the settings down.
  let aids = $state(loadReadingAids());

  function toggle(aid: keyof ReadingAids) {
    aids[aid] = !aids[aid];
    // When storage refuses, the choice still holds until the app closes.
    saveReadingAids(aids);
  }

  // Spotify, when this server has it set up: connecting goes through Spotify's own sign-in page and back.
  let spotify = $state<SpotifyStatus | null>(null);
  let spotifyNote = $state("");

  Spotify.Status()
    .then((s) => (spotify = s))
    .catch(() => (spotify = null));

  // YouTube, on a server: a song can play along with its video instead of Spotify.
  let youtube = $state<YouTubeStatus | null>(null);

  YouTube.Status()
    .then((s) => (youtube = s))
    .catch(() => (youtube = null));

  // An admin's YouTube Data API key, for songs to find their videos: checked with Google, then kept sealed.
  let ytKey = $state("");
  let ytBusy = $state(false);
  let ytNote = $state<{ ok: boolean; text: string } | null>(null);
  const ytKeyId = $props.id();

  async function saveKey() {
    ytBusy = true;
    ytNote = null;
    try {
      await YouTube.SetAPIKey(ytKey);
      ytKey = "";
      youtube = await YouTube.Status();
      ytNote = { ok: true, text: "Saved. Songs now find their videos on YouTube." };
    } catch (err) {
      ytNote = { ok: false, text: toAppError(err).message };
    } finally {
      ytBusy = false;
    }
  }

  async function clearKey() {
    ytBusy = true;
    ytNote = null;
    try {
      await YouTube.ClearAPIKey();
      youtube = await YouTube.Status();
    } catch (err) {
      ytNote = { ok: false, text: toAppError(err).message };
    } finally {
      ytBusy = false;
    }
  }

  async function disconnect() {
    spotifyNote = "";
    try {
      await Spotify.Disconnect();
      spotify = await Spotify.Status();
    } catch (err) {
      spotifyNote = toAppError(err).message;
    }
  }

  let busy = $state(false);
  let accountNote = $state<{ ok: boolean; text: string } | null>(null);

  async function add() {
    busy = true;
    accountNote = null;
    try {
      await addPasskey();
      rememberAccountHere();
      accountNote = { ok: true, text: "Passkey added. You can sign in with it on that device now." };
    } catch (err) {
      accountNote = { ok: false, text: toAppError(err).message };
    } finally {
      busy = false;
    }
  }

  async function out() {
    busy = true;
    try {
      await signOut();
      // Whoever signs in next starts at their own library, not on this screen.
      window.location.hash = homeHref;
      signedOut();
    } catch (err) {
      accountNote = { ok: false, text: toAppError(err).message };
      busy = false;
    }
  }

  // The admins' song tools, for testing, when the server has them on.
  let songTools = $state(false);
  Tools.Available()
    .then((on) => (songTools = on))
    .catch(() => (songTools = false));
</script>

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
    <div class="topbar-title"><h1>Settings</h1></div>
  </header>

  <main>
    <section>
      <h2>Reading aids</h2>
      <div class="aids"><ReadingAidSwitches {aids} ontoggle={toggle} /></div>
      <p class="muted small">What every song shows with its lyrics. A song's Aa menu changes the same choice.</p>
    </section>

    <section>
      <h2>Theme</h2>
      <div class="card"><ThemePicker /></div>
    </section>

    {#if spotify?.enabled}
      <section>
        <h2>Spotify</h2>
        <div class="card account">
          {#if spotify.connected}
            <p>Connected as <strong>{spotify.name}</strong></p>
            <p class="muted small">
              Songs have a Play button that plays them on your Spotify, or right here in the browser, and their lyrics
              follow along.
            </p>
            {#if !spotify.canPlayHere}
              <p class="note" role="status">
                Reconnect once so songs can play in this browser too, without opening the Spotify app.
              </p>
            {/if}
            <a class="button" class:primary={!spotify.canPlayHere} href="/auth/spotify/connect">Reconnect Spotify</a>
            <button type="button" class="button" onclick={disconnect}>Disconnect Spotify</button>
          {:else}
            <p class="muted small">
              Play songs on your Spotify, on this phone or any of your devices, and their lyrics follow along. Needs
              Spotify Premium.
            </p>
            <a class="button primary" href="/auth/spotify/connect">Connect Spotify</a>
          {/if}
          {#if spotifyNote}<p class="alert" role="alert">{spotifyNote}</p>{/if}
        </div>
      </section>
    {/if}

    {#if youtube?.enabled}
      <section>
        <h2>YouTube</h2>
        <div class="card account">
          <p class="small">
            Songs can also play along with their YouTube video, right here in the page. On a song, tap
            <strong>YouTube</strong> in the player bar, or choose it in the song's Aa menu under Play along with.
          </p>
          <p class="muted small">
            {youtube.searchable
              ? "Each song finds its video on YouTube by itself; you can paste a different link."
              : "Paste each song's YouTube or YouTube Music link. This server has no YouTube key to search with."}
          </p>
          {#if youtube.canSetKey}
            <form
              class="key"
              onsubmit={(event) => {
                event.preventDefault();
                void saveKey();
              }}
            >
              <label for={ytKeyId}>
                {youtube.keyFrom === "settings" ? "Replace the YouTube API key" : "YouTube Data API key"}
              </label>
              <input
                id={ytKeyId}
                type="password"
                autocomplete="off"
                spellcheck="false"
                placeholder="AIza…"
                bind:value={ytKey}
                disabled={ytBusy}
              />
              <p class="muted small">
                {youtube.keyFrom === "server"
                  ? "The server has a key of its own; one saved here is used instead."
                  : "Make one in Google Cloud for the YouTube Data API v3, or run make youtube-key on the server. It's checked with Google, then kept sealed on the server for everyone's searches."}
              </p>
              <button type="submit" class="button primary" disabled={ytBusy || ytKey.trim() === ""}>
                {ytBusy ? "Checking…" : "Save key"}
              </button>
            </form>
            {#if youtube.keyFrom === "settings"}
              <button type="button" class="button danger" disabled={ytBusy} onclick={clearKey}>Remove the key</button>
            {/if}
            {#if ytNote}
              <p class:alert={!ytNote.ok} class="note" role="status">{ytNote.text}</p>
            {/if}
          {/if}
        </div>
      </section>
    {/if}

    {#if account.me?.mode === "server" && account.me.user}
      <section>
        <h2>Account</h2>
        <div class="card account">
          <p>
            Signed in as <strong>{account.me.user}</strong>{#if account.me.admin}<span class="chip accent badge"
                >Admin</span
              >{/if}
          </p>
          {#if account.me.admin}
            <a class="button primary" href={invitesHref}>Manage invites</a>
            {#if songTools}
              <a class="button" href={toolsHref}>Song tools</a>
            {/if}
          {/if}
          <p class="muted small">Add a passkey on each device you use, so losing one doesn't lock you out.</p>
          <button type="button" class="button" disabled={busy} onclick={add}>Add a passkey on this device</button>
          <button type="button" class="button danger" disabled={busy} onclick={out}>Sign out</button>
          {#if accountNote}
            <p class:alert={!accountNote.ok} class="note" role="status">{accountNote.text}</p>
          {/if}
        </div>
      </section>
    {/if}

    <section>
      <a class="nav-row" href={aboutHref}>
        <span class="text">
          <span class="label">About Utanabi</span>
          <span class="hint">Version, sources, and credits</span>
        </span>
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"><polyline points="9 18 15 12 9 6" /></svg
        >
      </a>
    </section>
  </main>
</div>

<style>
  main {
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 12px 0 max(32px, env(safe-area-inset-bottom));
  }

  p {
    margin: 0;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .aids {
    --group-bg: var(--surface);
  }

  .small {
    margin-top: 8px;
    font-size: 0.875rem;
  }

  .card {
    padding: 8px;
    border-radius: 16px;
    background: var(--surface);
  }

  .account {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px;
  }

  .account .small {
    margin-top: 0;
  }

  .account .button {
    flex: none;
  }

  .key {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .key label {
    font-size: 0.875rem;
    font-weight: 600;
  }

  .key input {
    min-height: var(--tap);
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg);
    color: var(--fg);
    font: inherit;
    font-size: 1rem;
  }

  .badge {
    margin-left: 8px;
    vertical-align: middle;
  }

  .nav-row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 60px;
    padding: 8px 12px 8px 16px;
    border-radius: 16px;
    background: var(--surface);
    color: var(--fg);
    text-decoration: none;
  }

  .nav-row:hover {
    color: var(--fg);
    background: var(--surface-strong);
  }

  .nav-row svg {
    flex: none;
    color: var(--muted);
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
</style>
