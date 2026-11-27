<script lang="ts">
  import { account, loadAccount, signedOut } from "./lib/account.svelte";
  import { whenSignedOut } from "./lib/errors";
  import { homeHref, parseRoute } from "./lib/route";
  import About from "./screens/About.svelte";
  import AddSong from "./screens/AddSong.svelte";
  import Home from "./screens/Home.svelte";
  import Invites from "./screens/Invites.svelte";
  import Quiz from "./screens/Quiz.svelte";
  import Search from "./screens/Search.svelte";
  import Settings from "./screens/Settings.svelte";
  import SignIn from "./screens/SignIn.svelte";
  import SongView from "./screens/SongView.svelte";
  import Study from "./screens/Study.svelte";

  let route = $state(parseRoute(window.location.hash));

  // On a server with accounts, nothing shows until someone signs in; a call that finds the session ended comes back
  // here. The desktop and phone apps have no accounts and go straight in.
  void loadAccount();
  whenSignedOut(signedOut);

  $effect(() => {
    const onHashChange = () => {
      route = parseRoute(window.location.hash);
      window.scrollTo(0, 0);
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  });
</script>

{#if account.me === null}
  <main class="screen missing"><p class="muted">Loading…</p></main>
{:else if account.me.mode === "server" && !account.me.user}
  <SignIn canSignUp={account.me.signUp} invite={route.name === "join" ? route.code : ""} />
{:else if route.name === "home" || route.name === "join"}
  <Home />
{:else if route.name === "invites"}
  <Invites />
{:else if route.name === "search"}
  <Search />
{:else if route.name === "settings"}
  <Settings />
{:else if route.name === "about"}
  <About />
{:else if route.name === "add"}
  <AddSong />
{:else if route.name === "song"}
  {#key route.id}
    <SongView id={route.id} />
  {/key}
{:else if route.name === "study"}
  <!-- Keyed by song only, so stepping from card to card keeps the deck. -->
  {#key route.id}
    <Study songId={route.id} cardId={route.card} />
  {/key}
{:else if route.name === "quiz"}
  {#key route.id}
    <Quiz songId={route.id} />
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
