<script lang="ts">
  import { signedIn } from "../lib/account.svelte";
  import {
    beginSignUp,
    canAutofillPasskeys,
    finishSignUp,
    hadAccountHere,
    rememberAccountHere,
    signIn,
    wasStopped,
    type CreationJSON,
  } from "../lib/auth";
  import { toAppError, type AppError } from "../lib/errors";

  // The way in on a server with accounts: a passkey (Face ID, a fingerprint, or the device's PIN), no password. A
  // first visit leads with making an account, which saves a passkey on this device in one step. A browser that has
  // signed in before leads with signing in, and its passkeys show in the username field's autofill as well.
  // invite comes from an invite link (#/join/<code>), which leads with making an account, with the code filled in.
  let { canSignUp, invite = "" }: { canSignUp: boolean; invite?: string } = $props();

  let returning = $state(hadAccountHere());
  let signUpFirst = $derived(canSignUp && (!returning || invite !== ""));

  let name = $state("");
  let code = $state("");
  // An invite link fills the code in.
  $effect(() => {
    if (invite) {
      code = invite;
    }
  });
  // After Continue: the passkey to save, waiting for its own tap.
  let ready = $state<CreationJSON | null>(null);
  let busy = $state(false);
  let failure = $state<AppError | null>(null);
  let background: AbortController | null = null;
  let autofill = false;

  let fieldError = $derived(failure?.code === "invalid_input" ? failure.field : undefined);

  // A sign-in that waits in the background for a passkey picked from the username field's autofill. Only on a
  // browser that has signed in before: Safari won't save a new passkey while such a sign-in is waiting, so a first
  // visit, which is for making an account, never starts one.
  $effect(() => {
    let live = true;
    void canAutofillPasskeys().then((ok) => {
      autofill = ok && returning;
      if (autofill && live) {
        startBackground();
      }
    });
    return () => {
      live = false;
      background?.abort();
    };
  });

  function startBackground() {
    background?.abort();
    const controller = new AbortController();
    background = controller;
    signIn({ signal: controller.signal })
      .then(done)
      // Stopped for a prompt, or nothing picked: nothing to show.
      .catch(() => {});
  }

  function done(user: string) {
    rememberAccountHere();
    void signedIn(user);
  }

  async function run(step: () => Promise<string>) {
    background?.abort();
    busy = true;
    failure = null;
    try {
      done(await step());
    } catch (err) {
      if (!wasStopped(err)) {
        failure = toAppError(err);
      }
      // Back to waiting for a passkey from autofill.
      if (autofill) {
        startBackground();
      }
    } finally {
      busy = false;
    }
  }

  // Continue: check the username and code, then wait for the tap that saves the passkey.
  async function next(event: SubmitEvent) {
    event.preventDefault();
    background?.abort();
    busy = true;
    failure = null;
    try {
      ready = await beginSignUp(name, code);
    } catch (err) {
      failure = toAppError(err);
    } finally {
      busy = false;
    }
  }

  function save() {
    const options = ready;
    if (options) {
      void run(() => finishSignUp(options));
    }
  }
</script>

<!-- prettier-ignore -->
{#snippet signInPart()}
  <section class="part" aria-labelledby="signin-title">
    <h2 id="signin-title">{signUpFirst ? "Already have an account?" : "Welcome back"}</h2>
    <label for="signin-name">Username</label>
    <input
      id="signin-name"
      autocomplete="username webauthn"
      autocapitalize="off"
      autocorrect="off"
      spellcheck="false"
      aria-describedby="signin-hint"
    />
    <p id="signin-hint" class="hint">Tap here and pick your passkey, or use the button.</p>
    <button type="button" class="button" class:primary={!signUpFirst} disabled={busy} onclick={() => void run(signIn)}>
      {busy ? "Waiting for your passkey…" : "Sign in with a passkey"}
    </button>
  </section>
{/snippet}

<!-- prettier-ignore -->
{#snippet signUpPart()}
  {#if ready}
    <section class="part" aria-labelledby="save-title">
      <h2 id="save-title">One more step</h2>
      <p class="steps">
        Save a passkey for <strong>{name.trim().toLowerCase()}</strong> on this device. Your phone asks for Face ID,
        your fingerprint, or its PIN.
      </p>
      <button type="button" class="button primary" disabled={busy} onclick={save}>
        {busy ? "Waiting for your passkey…" : "Save passkey"}
      </button>
      <button type="button" class="link" disabled={busy} onclick={() => (ready = null)}>Change username</button>
    </section>
  {:else}
  <form class="part" onsubmit={next} aria-labelledby="signup-title">
    <h2 id="signup-title">{signUpFirst ? "Make your account" : "New here?"}</h2>
    <p class="steps">
      {invite
        ? "Your invite is filled in. Pick a username, then your phone or computer saves a passkey with Face ID, your fingerprint, or its PIN. There's no password and no QR code to scan."
        : "Pick a username and enter the invite code you were given. Then your phone or computer saves a passkey with Face ID, your fingerprint, or its PIN. There's no password and no QR code to scan."}
    </p>
    <label for="signup-name">Choose a username</label>
    <input
      id="signup-name"
      bind:value={name}
      autocomplete="username"
      autocapitalize="off"
      autocorrect="off"
      spellcheck="false"
      required
      minlength="3"
      maxlength="32"
      aria-describedby="signup-name-hint"
      aria-invalid={fieldError === "name"}
    />
    <p id="signup-name-hint" class="hint">3 to 32 letters, numbers, _ or -.</p>
    {#if fieldError === "name"}<p class="error" role="alert">{failure?.message}</p>{/if}

    <label for="signup-code">Invite code</label>
    <input
      id="signup-code"
      bind:value={code}
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck="false"
      required
      aria-invalid={fieldError === "code" || fieldError === "signup"}
    />
    {#if fieldError === "code" || fieldError === "signup"}<p class="error" role="alert">{failure?.message}</p>{/if}

    <button type="submit" class="button" class:primary={signUpFirst} disabled={busy}>
      {busy ? "Checking…" : "Continue"}
    </button>
  </form>
  {/if}
{/snippet}

<div class="screen">
  <main>
    <header>
      <p class="mark" lang="ja">うたなび</p>
      <h1>{signUpFirst ? "Welcome to Utanabi" : "Sign in to Utanabi"}</h1>
      <p class="muted">With a passkey: Face ID, your fingerprint, or your device's PIN. There's no password.</p>
    </header>

    {#if failure && fieldError === undefined}
      <p class="alert" role="alert">{failure.message}</p>
    {/if}

    {#if signUpFirst}
      {@render signUpPart()}
      {@render signInPart()}
    {:else}
      {@render signInPart()}
      {#if canSignUp}
        {@render signUpPart()}
      {:else}
        <p class="muted closed">This server isn't taking new accounts.</p>
      {/if}
    {/if}
  </main>
</div>

<style>
  main {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 16px;
    padding: max(32px, env(safe-area-inset-top)) 0 max(32px, env(safe-area-inset-bottom));
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 6px;
    text-align: center;
  }

  header p,
  h1,
  h2,
  .steps {
    margin: 0;
  }

  .mark {
    font-size: 1.75rem;
    font-weight: 800;
  }

  h1 {
    font-size: 1.5rem;
  }

  .part {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 20px 16px;
    border-radius: 20px;
    background: var(--surface);
  }

  h2 {
    margin-bottom: 4px;
    font-size: 1.0625rem;
  }

  .steps {
    margin-bottom: 8px;
    font-size: 0.875rem;
    color: var(--muted);
  }

  label {
    font-weight: 600;
  }

  input {
    min-height: 48px;
    margin-bottom: 4px;
    padding: 0 14px;
    border: 1px solid transparent;
    border-radius: 12px;
    background: var(--bg);
    color: var(--fg);
    font-size: 1rem;
  }

  input:focus {
    border-color: var(--link);
    outline: none;
  }

  .hint {
    margin: -2px 0 6px;
    font-size: 0.8125rem;
    color: var(--muted);
  }

  .part .button {
    flex: none;
    margin-top: 8px;
  }

  .closed {
    text-align: center;
  }

  .link {
    min-height: var(--tap);
    border: 0;
    background: transparent;
    color: var(--link);
    font-weight: 600;
    text-decoration: underline;
    cursor: pointer;
  }
</style>
