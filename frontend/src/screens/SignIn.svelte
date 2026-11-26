<script lang="ts">
  import { signedIn } from "../lib/account.svelte";
  import { signIn, signUp } from "../lib/auth";
  import { toAppError, type AppError } from "../lib/errors";

  // The way in on a server with accounts: a passkey (Face ID, a fingerprint, or the device's PIN), no password. A new
  // account needs a username and the server's sign-up code.
  let { canSignUp }: { canSignUp: boolean } = $props();

  let name = $state("");
  let code = $state("");
  let busy = $state(false);
  let failure = $state<AppError | null>(null);

  let fieldError = $derived(failure?.code === "invalid_input" ? failure.field : undefined);

  async function run(step: () => Promise<string>) {
    busy = true;
    failure = null;
    try {
      signedIn(await step());
    } catch (err) {
      failure = toAppError(err);
    } finally {
      busy = false;
    }
  }

  function create(event: SubmitEvent) {
    event.preventDefault();
    void run(() => signUp(name, code));
  }
</script>

<div class="screen">
  <main>
    <header>
      <p class="mark" lang="ja">うたなび</p>
      <h1>Sign in to Utanabi</h1>
      <p class="muted">With a passkey: Face ID, your fingerprint, or your device's PIN. There's no password.</p>
    </header>

    <button type="button" class="button primary" disabled={busy} onclick={() => void run(signIn)}>
      {busy ? "Waiting for your passkey…" : "Sign in with a passkey"}
    </button>

    {#if failure && fieldError === undefined}
      <p class="alert" role="alert">{failure.message}</p>
    {/if}

    {#if canSignUp}
      <form class="new" onsubmit={create}>
        <h2>New here?</h2>
        <label for="signup-name">Username</label>
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

        <button type="submit" class="button" disabled={busy}>Create an account with a passkey</button>
      </form>
    {:else}
      <p class="muted closed">This server isn't taking new accounts.</p>
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
  h2 {
    margin: 0;
  }

  .mark {
    font-size: 1.75rem;
    font-weight: 800;
  }

  h1 {
    font-size: 1.5rem;
  }

  .button {
    flex: none;
  }

  .new {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 12px;
    padding: 20px 16px;
    border-radius: 20px;
    background: var(--surface);
  }

  h2 {
    margin-bottom: 8px;
    font-size: 1.0625rem;
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

  .new .button {
    margin-top: 10px;
  }

  .closed {
    text-align: center;
  }
</style>
