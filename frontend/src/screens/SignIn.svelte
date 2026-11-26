<script lang="ts">
  import { signedIn } from "../lib/account.svelte";
  import { canAutofillPasskeys, hadAccountHere, rememberAccountHere, signIn, signUp, wasStopped } from "../lib/auth";
  import { toAppError, type AppError } from "../lib/errors";

  // The way in on a server with accounts: a passkey (Face ID, a fingerprint, or the device's PIN), no password. A
  // first visit leads with making an account, which saves a passkey on this device in one step. A browser that has
  // signed in before leads with signing in, and its passkeys show in the username field's autofill as well.
  let { canSignUp }: { canSignUp: boolean } = $props();

  let returning = $state(hadAccountHere());
  let signUpFirst = $derived(canSignUp && !returning);

  let name = $state("");
  let code = $state("");
  let busy = $state(false);
  let failure = $state<AppError | null>(null);
  let background: AbortController | null = null;
  let autofill = false;

  let fieldError = $derived(failure?.code === "invalid_input" ? failure.field : undefined);

  // A sign-in that waits in the background for a passkey picked from the username field's autofill.
  $effect(() => {
    let live = true;
    void canAutofillPasskeys().then((ok) => {
      autofill = ok;
      if (ok && live) {
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
    signedIn(user);
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

  function create(event: SubmitEvent) {
    event.preventDefault();
    void run(() => signUp(name, code));
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
  <form class="part" onsubmit={create} aria-labelledby="signup-title">
    <h2 id="signup-title">{signUpFirst ? "Make your account" : "New here?"}</h2>
    <p class="steps">
      Pick a username and enter the sign-up code you were given. Your phone or computer then saves a passkey with Face
      ID, your fingerprint, or its PIN. There's no password and no QR code to scan.
    </p>
    <label for="signup-name">Choose a username</label>
    <input
      id="signup-name"
      bind:value={name}
      autocomplete="username"
      autocapitalize="off"
      spellcheck="false"
      required
      minlength="3"
      maxlength="32"
      aria-describedby="signup-name-hint"
      aria-invalid={fieldError === "name"}
    />
    <p id="signup-name-hint" class="hint">3 to 32 letters, numbers, _ or -.</p>
    {#if fieldError === "name"}<p class="error" role="alert">{failure?.message}</p>{/if}

    <label for="signup-code">Sign-up code</label>
    <input
      id="signup-code"
      bind:value={code}
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
      required
      aria-invalid={fieldError === "code" || fieldError === "signup"}
    />
    {#if fieldError === "code" || fieldError === "signup"}<p class="error" role="alert">{failure?.message}</p>{/if}

    <button type="submit" class="button" class:primary={signUpFirst} disabled={busy}>
      Create an account with a passkey
    </button>
  </form>
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
</style>
