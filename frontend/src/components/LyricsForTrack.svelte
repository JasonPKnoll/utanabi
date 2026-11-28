<script lang="ts">
  import { Service as Songs, type Match } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/songs";
  import { toAppError } from "../lib/errors";
  import { songHref } from "../lib/route";
  import { formatDuration } from "../lib/sources";
  import { forTrack, timedForAnother } from "../lib/versions";

  // When the song's lyrics are timed for another version than the track playing, looks for lyrics timed for the
  // track, and offers them: they line up better than any offset, since hand-timed lyrics differ line by line.
  let { songId, lyricsMs, trackMs, query }: { songId: number; lyricsMs: number; trackMs: number; query: string } =
    $props();

  let found = $state<Match | null>(null);
  let looked = $state(false);
  let busy = $state(false);
  let note = $state("");

  $effect(() => {
    if (!timedForAnother(lyricsMs, trackMs)) {
      return;
    }
    Songs.Search(query)
      .then((result) => (found = forTrack(result.matches ?? [], trackMs, songId)))
      .catch(() => (found = null))
      .finally(() => (looked = true));
  });

  async function add(match: Match) {
    busy = true;
    note = "";
    try {
      const song = await Songs.AddFromSource(match.source, match.sourceRef);
      window.location.hash = songHref(song.id);
    } catch (err) {
      note = toAppError(err).message;
      busy = false;
    }
  }
</script>

{#if timedForAnother(lyricsMs, trackMs) && looked}
  <div class="versions">
    <h3>Lyrics for this track</h3>
    <p class="muted">
      These lyrics are timed for a {formatDuration(lyricsMs)} version; the track playing is {formatDuration(trackMs)}.
    </p>
    {#if found}
      <p>
        {found.sourceName} has lyrics timed for {formatDuration(found.durationMs)}, which should line up with no offset.
      </p>
      {#if found.songId > 0}
        <a class="button primary" href={songHref(found.songId)}>Open them</a>
      {:else}
        <button type="button" class="button primary" disabled={busy} onclick={() => found && add(found)}>
          {busy ? "Adding…" : "Add them"}
        </button>
      {/if}
    {:else}
      <p class="muted">No lyrics timed for this track were found, so the offset above is the way to line these up.</p>
    {/if}
    {#if note}<p class="alert" role="alert">{note}</p>{/if}
  </div>
{/if}

<style>
  .versions {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: 24px;
  }

  h3 {
    margin: 0;
    font-size: 0.8125rem;
    font-weight: 650;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }

  p {
    margin: 0;
    font-size: 0.9375rem;
  }

  .button {
    width: 100%;
    margin-top: 4px;
  }
</style>
