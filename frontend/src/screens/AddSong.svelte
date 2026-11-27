<script lang="ts">
  import { Service as Songs } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import SongBuilding from "../components/SongBuilding.svelte";
  import { toAppError, type AppError } from "../lib/errors";
  import { homeHref, songHref } from "../lib/route";

  let title = $state("");
  let artist = $state("");
  let lyrics = $state("");
  let saving = $state(false);
  let failure = $state<AppError | null>(null);

  function fieldError(field: string): string | null {
    return failure?.code === "invalid_input" && failure.field === field ? failure.message : null;
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (saving) {
      return;
    }
    saving = true;
    failure = null;
    try {
      const song = await Songs.Add({ title, artist, lyrics });
      // Replace the form in history, so Back from the new song goes to the list.
      window.location.replace(songHref(song.id));
    } catch (err) {
      failure = toAppError(err);
    } finally {
      saving = false;
    }
  }
</script>

{#if saving}
  <SongBuilding title={title.trim()} />
{/if}

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
    <div class="topbar-title"><h1>Add your own lyrics</h1></div>
  </header>

  <form onsubmit={save} novalidate>
    {#if failure && failure.code !== "invalid_input"}
      <p class="alert" role="alert">{failure.message}</p>
    {/if}

    <div class="field">
      <label for="song-title">Song title</label>
      <input
        id="song-title"
        bind:value={title}
        autocomplete="off"
        aria-invalid={fieldError("title") !== null}
        aria-describedby={fieldError("title") ? "song-title-error" : undefined}
      />
      {#if fieldError("title")}
        <p id="song-title-error" class="error">{fieldError("title")}</p>
      {/if}
    </div>

    <div class="field">
      <label for="song-artist">Artist <span class="muted">(optional)</span></label>
      <input
        id="song-artist"
        bind:value={artist}
        autocomplete="off"
        aria-invalid={fieldError("artist") !== null}
        aria-describedby={fieldError("artist") ? "song-artist-error" : undefined}
      />
      {#if fieldError("artist")}
        <p id="song-artist-error" class="error">{fieldError("artist")}</p>
      {/if}
    </div>

    <div class="field grow">
      <label for="song-lyrics">Lyrics</label>
      <p id="song-lyrics-hint" class="hint">
        Paste or type the song, one sung line per line. Leave a blank line between verses.
      </p>
      <textarea
        id="song-lyrics"
        bind:value={lyrics}
        rows="12"
        lang="ja"
        aria-invalid={fieldError("lyrics") !== null}
        aria-describedby={fieldError("lyrics") ? "song-lyrics-hint song-lyrics-error" : "song-lyrics-hint"}></textarea>
      {#if fieldError("lyrics")}
        <p id="song-lyrics-error" class="error">{fieldError("lyrics")}</p>
      {/if}
    </div>

    <div class="actions">
      <button class="button primary" type="submit" disabled={saving}>{saving ? "Saving…" : "Save song"}</button>
    </div>
  </form>
</div>

<style>
  form {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label {
    font-weight: 600;
  }

  .hint {
    margin: 0;
    font-size: 0.9rem;
    color: var(--muted);
  }

  input,
  textarea {
    box-sizing: border-box;
    width: 100%;
    min-height: 50px;
    padding: 12px 16px;
    border: 1px solid transparent;
    border-radius: 14px;
    background: var(--surface);
    color: var(--fg);
    font-size: 1rem;
  }

  input:focus,
  textarea:focus {
    border-color: var(--link);
    outline: none;
  }

  textarea {
    min-height: 260px;
    resize: vertical;
    line-height: 1.8;
  }

  input[aria-invalid="true"],
  textarea[aria-invalid="true"] {
    border-color: var(--danger);
  }
</style>
