<script lang="ts">
  import {
    Service as Accounts,
    type Invite,
    type NewInvite,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/accounts";
  import { toAppError, type AppError } from "../lib/errors";
  import { joinHref, settingsHref } from "../lib/route";

  // For admins: make an invite link to share, and see and revoke the ones made. A link makes one account, within a
  // week. Its code is shown once, here, right after it's made; the server keeps only its hash.
  let invites = $state<Invite[] | null>(null);
  let failure = $state<AppError | null>(null);
  let made = $state<NewInvite | null>(null);
  let busy = $state(false);
  let copied = $state(false);

  let link = $derived(made ? new URL(joinHref(made.code), window.location.href).toString() : "");
  let canShare = typeof navigator.share === "function";

  function load() {
    Accounts.Invites()
      .then((list) => (invites = list ?? []))
      .catch((err: unknown) => (failure = toAppError(err)));
  }
  load();

  async function make() {
    busy = true;
    failure = null;
    copied = false;
    try {
      made = await Accounts.CreateInvite();
      load();
    } catch (err) {
      failure = toAppError(err);
    } finally {
      busy = false;
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      copied = true;
    } catch {
      failure = { code: "internal", message: "Couldn't copy it. Press and hold the link to copy it instead." };
    }
  }

  async function share() {
    try {
      await navigator.share({ title: "Join me on Utanabi", text: "An invite to Utanabi, for one account.", url: link });
    } catch (err) {
      // Closing the share sheet is no failure.
      if (!(err instanceof DOMException && err.name === "AbortError")) {
        failure = { code: "internal", message: "Couldn't share it. Copy the link instead." };
      }
    }
  }

  async function revoke(id: number) {
    failure = null;
    try {
      await Accounts.RevokeInvite(id);
      if (made?.invite.id === id) {
        made = null;
      }
      load();
    } catch (err) {
      failure = toAppError(err);
    }
  }

  function when(iso: string): string {
    return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  }

  function status(inv: Invite): string {
    if (inv.usedBy) {
      return `Used by ${inv.usedBy} on ${when(inv.usedAt)}`;
    }
    if (new Date(inv.expiresAt).getTime() <= Date.now()) {
      return `Ran out ${when(inv.expiresAt)}`;
    }
    return `Unused, works until ${when(inv.expiresAt)}`;
  }

  function usable(inv: Invite): boolean {
    return !inv.usedBy && new Date(inv.expiresAt).getTime() > Date.now();
  }
</script>

<div class="screen">
  <header class="topbar">
    <a class="icon-button" href={settingsHref} aria-label="Back to settings">
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
    <div class="topbar-title"><h1>Invites</h1></div>
  </header>

  <main>
    {#if failure?.code === "forbidden"}
      <p class="alert" role="alert">Only an admin can manage invites.</p>
    {:else}
      <section class="card make">
        <p class="muted">An invite link makes one account, within a week. Send it to someone you want to join.</p>
        <button type="button" class="button primary" disabled={busy} onclick={make}>
          {busy ? "Making…" : "Make an invite link"}
        </button>

        {#if made}
          <div class="made" role="status">
            <p class="label">Your invite link</p>
            <p class="link" data-testid="invite-link">{link}</p>
            <p class="muted small">This is the only time it's shown. It works once.</p>
            <div class="row">
              {#if canShare}
                <button type="button" class="button primary" onclick={share}>Share</button>
              {/if}
              <button type="button" class="button" onclick={copy}>{copied ? "Copied" : "Copy link"}</button>
            </div>
          </div>
        {/if}
      </section>

      {#if failure}
        <p class="alert" role="alert">{failure.message}</p>
      {/if}

      <section>
        <h2>All invites</h2>
        {#if invites === null}
          <p class="muted">Loading…</p>
        {:else if invites.length === 0}
          <p class="muted">None yet.</p>
        {:else}
          <ul class="list">
            {#each invites as inv (inv.id)}
              <li>
                <span class="text">
                  <span class="title">Made {when(inv.createdAt)}{inv.createdBy ? ` by ${inv.createdBy}` : ""}</span>
                  <span class="sub">{status(inv)}</span>
                </span>
                {#if usable(inv)}
                  <button type="button" class="button danger revoke" onclick={() => revoke(inv.id)}>Revoke</button>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {/if}
  </main>
</div>

<style>
  main {
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 12px 0 max(32px, env(safe-area-inset-bottom));
  }

  p {
    margin: 0;
  }

  .card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    border-radius: 20px;
    background: var(--surface);
  }

  .button {
    flex: none;
  }

  .made {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 12px;
    border-top: 1px solid var(--surface-strong);
  }

  .label {
    font-weight: 650;
  }

  .link {
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--bg);
    color: var(--link);
    font-size: 0.875rem;
    overflow-wrap: anywhere;
    user-select: all;
  }

  .small {
    font-size: 0.8125rem;
  }

  .row {
    display: flex;
    gap: 8px;
  }

  .row .button {
    flex: 1;
  }

  h2 {
    margin: 0 0 8px;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .list {
    margin: 0;
    padding: 0;
    list-style: none;
    border-radius: 16px;
    background: var(--surface);
    overflow: hidden;
  }

  .list li {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 60px;
    padding: 8px 12px 8px 16px;
  }

  .list li + li {
    border-top: 1px solid var(--surface-strong);
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .title {
    font-weight: 600;
  }

  .sub {
    font-size: 0.8125rem;
    color: var(--muted);
  }

  .revoke {
    min-height: var(--tap);
    padding: 0 14px;
  }
</style>
