<script lang="ts">
  import { AboutService, type About } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import ThemePicker from "../components/ThemePicker.svelte";
  import { account, signedOut } from "../lib/account.svelte";
  import { addPasskey, rememberAccountHere, signOut } from "../lib/auth";
  import { toAppError, type AppError } from "../lib/errors";
  import { homeHref, invitesHref } from "../lib/route";

  // The app's version and the full credits its data asks for. Other screens credit JMdict in one quiet line and link
  // here, which is where the licences ask an app to say the rest.
  let about = $state<About | null>(null);
  let failure = $state<AppError | null>(null);

  AboutService.Info()
    .then((info) => (about = info))
    .catch((err: unknown) => (failure = toAppError(err)));

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
    <div class="topbar-title"><h1>About</h1></div>
  </header>

  <main>
    {#if failure}
      <p class="alert" role="alert">{failure.message}</p>
    {:else if about}
      <section class="app">
        <p class="mark" lang="ja">うたなび</p>
        <p class="muted">Learn Japanese through song lyrics.</p>
        <p class="version" data-testid="about">{about.name} {about.version} · {about.platform}</p>
      </section>

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
        <h2>Theme</h2>
        <div class="card"><ThemePicker /></div>
      </section>

      <section>
        <h2>Sources</h2>
        <ul class="sources">
          <li>
            <p class="name">LRCLIB</p>
            <p class="muted">Song search and lyrics. A free, community-run lyrics database at lrclib.net.</p>
          </li>
          {#each about.credits ?? [] as credit (credit.name)}
            <li>
              <p class="name">{credit.name}</p>
              <p class="muted">{credit.text}</p>
            </li>
          {/each}
        </ul>
      </section>
    {:else}
      <p class="muted">Loading…</p>
    {/if}
  </main>
</div>

<style>
  main {
    display: flex;
    flex-direction: column;
    gap: 28px;
    padding: 12px 0 max(32px, env(safe-area-inset-bottom));
  }

  p {
    margin: 0;
  }

  .app {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 20px 16px;
    border-radius: 20px;
    background: var(--surface);
    text-align: center;
  }

  .mark {
    font-size: 1.75rem;
    font-weight: 800;
  }

  .version {
    margin-top: 6px;
    font-size: 0.8125rem;
    color: var(--muted);
  }

  h2 {
    margin: 0 0 8px;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .account {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px !important;
  }

  .account p {
    margin: 0;
  }

  .account .button {
    flex: none;
  }

  .small {
    font-size: 0.875rem;
  }

  .badge {
    margin-left: 8px;
    vertical-align: middle;
  }

  .card {
    padding: 8px;
    border-radius: 16px;
    background: var(--surface);
  }

  .sources {
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--surface);
    overflow: hidden;
  }

  .sources li {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 14px 16px;
    font-size: 0.875rem;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .sources li + li {
    border-top: 1px solid var(--surface-strong);
  }

  .name {
    font-size: 1rem;
    font-weight: 600;
  }
</style>
