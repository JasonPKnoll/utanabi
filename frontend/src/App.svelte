<script lang="ts">
  import { homeHref, parseRoute } from "./lib/route";
  import AddSong from "./screens/AddSong.svelte";
  import Home from "./screens/Home.svelte";
  import SongView from "./screens/SongView.svelte";

  let route = $state(parseRoute(window.location.hash));

  $effect(() => {
    const onHashChange = () => {
      route = parseRoute(window.location.hash);
      window.scrollTo(0, 0);
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  });
</script>

{#if route.name === "home"}
  <Home />
{:else if route.name === "add"}
  <AddSong />
{:else if route.name === "song"}
  {#key route.id}
    <SongView id={route.id} />
  {/key}
{:else}
  <main class="screen missing">
    <h1>Nothing here</h1>
    <p class="muted">This page doesn't exist.</p>
    <a class="button primary" href={homeHref}>Back to your songs</a>
  </main>
{/if}

<style>
  .missing {
    justify-content: center;
    gap: 12px;
    text-align: center;
  }

  .missing h1,
  .missing p {
    margin: 0;
  }

  .missing .button {
    flex: none;
    margin-top: 12px;
  }
</style>
