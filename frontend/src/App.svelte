<script lang="ts">
  import { AboutService, type About } from "../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import { toAppError, type AppError } from "./lib/errors";

  let about = $state<About | null>(null);
  let failure = $state<AppError | null>(null);

  AboutService.Info()
    .then((info) => (about = info))
    .catch((err: unknown) => (failure = toAppError(err)));
</script>

<main>
  <h1>
    <ruby>歌学び<rt>うたなび</rt></ruby>
    <span class="latin">Utanabi</span>
  </h1>
  <p class="tagline">Learn Japanese through song lyrics.</p>

  {#if failure}
    <p class="status error" role="alert">{failure.message}</p>
  {:else if about}
    <p class="status" data-testid="about">{about.name} {about.version} · {about.platform}</p>
  {:else}
    <p class="status">Loading…</p>
  {/if}
</main>

<style>
  main {
    min-height: 100dvh;
    display: grid;
    place-content: center;
    gap: 0.75rem;
    padding: max(16px, env(safe-area-inset-top)) 16px max(16px, env(safe-area-inset-bottom));
    text-align: center;
  }

  h1 {
    margin: 0;
    font-size: clamp(2.5rem, 10vw, 4rem);
    font-weight: 600;
    line-height: 1.4;
  }

  rt {
    font-size: 0.35em;
    color: var(--muted);
  }

  .latin {
    display: block;
    font-size: 0.4em;
    letter-spacing: 0.2em;
    text-transform: uppercase;
    color: var(--muted);
  }

  .tagline {
    margin: 0;
    font-size: 1.1rem;
  }

  .status {
    margin: 1rem 0 0;
    font-size: 0.9rem;
    color: var(--muted);
  }

  .error {
    color: var(--danger);
  }
</style>
