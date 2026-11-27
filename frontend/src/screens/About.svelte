<script lang="ts">
  import Wordmark from "../components/Wordmark.svelte";
  import { AboutService, type About } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/app";
  import { toAppError, type AppError } from "../lib/errors";
  import { placeholderAbout } from "../lib/placeholders";
  import { settingsHref } from "../lib/route";

  // The app's version and the full credits its data asks for, reached from Settings. Other screens credit JMdict in
  // one quiet line and link here, which is where the licences ask an app to say the rest.
  let about = $state<About | null>(null);
  let failure = $state<AppError | null>(null);

  AboutService.Info()
    .then((info) => (about = info))
    .catch((err: unknown) => (failure = toAppError(err)));

  // While it loads, stand-in credits go through the same markup, as a skeleton in the screen's shape.
  let loading = $derived(about === null);
  let shown = $derived(about ?? placeholderAbout);
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
    <div class="topbar-title"><h1>About</h1></div>
  </header>

  <main>
    {#if failure}
      <p class="alert" role="alert">{failure.message}</p>
    {:else}
      {#if loading}<p class="visually-hidden" role="status">Loading</p>{/if}
      <div class="content" class:skeleton-wait={loading} inert={loading} aria-hidden={loading ? "true" : undefined}>
        <section class="app">
          <p class="mark"><Wordmark size="2.25rem" /></p>
          <p class="muted">Learn Japanese through song lyrics.</p>
          <p class="version" data-testid={loading ? undefined : "about"}>
            <span class:ph={loading}>{shown.name} {shown.version} · {shown.platform}</span>
          </p>
        </section>

        <section>
          <h2>Sources</h2>
          <ul class="sources">
            <li>
              <p class="name">LRCLIB</p>
              <p class="muted">Song search and lyrics. A free, community-run lyrics database at lrclib.net.</p>
            </li>
            {#each shown.credits ?? [] as credit (credit.name)}
              <li>
                <p class="name"><span class:ph={loading}>{credit.name}</span></p>
                <p class="muted"><span class:ph={loading}>{credit.text}</span></p>
              </li>
            {/each}
          </ul>
        </section>
      </div>
    {/if}
  </main>
</div>

<style>
  main {
    padding: 12px 0 max(32px, env(safe-area-inset-bottom));
  }

  .content {
    display: flex;
    flex-direction: column;
    gap: 28px;
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
    line-height: 1;
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
