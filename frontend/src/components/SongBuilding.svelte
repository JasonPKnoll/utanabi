<script lang="ts">
  import Firework from "./Firework.svelte";

  // Covers the screen while a song is being built. It shows at once, and the screens that use it keep it up for at
  // least buildingAtLeastMs, so even a fast save shows one full bloom.
  let { title = "" }: { title?: string } = $props();
</script>

<div class="building" role="status">
  <Firework size={220} />
  <p class="title">{title ? `Building ${title}` : "Building your song"}</p>
  <p class="muted">Splitting the lyrics into words, adding readings, and finding each word's meaning.</p>
</div>

<style>
  .building {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: max(24px, env(safe-area-inset-top)) max(28px, env(safe-area-inset-right))
      max(24px, env(safe-area-inset-bottom)) max(28px, env(safe-area-inset-left));
    background: var(--bg);
    text-align: center;
    animation: appear 0.15s ease-out both;
  }

  .building :global(.stage) {
    margin-bottom: 14px;
  }

  p {
    margin: 0;
    max-width: 320px;
  }

  .title {
    font-size: 1.5rem;
    font-weight: 700;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  @keyframes appear {
    from {
      opacity: 0;
    }
  }
</style>
