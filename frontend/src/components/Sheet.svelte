<script lang="ts">
  import type { Snippet } from "svelte";

  // A sheet that slides up from the bottom. It's a native <dialog> opened with showModal, so focus stays inside it,
  // Escape closes it, and focus goes back to whatever opened it. A tap on the dimmed backdrop closes it too.
  // children gets a close function for its own buttons.
  let {
    labelledby,
    onclose,
    children,
  }: {
    labelledby: string;
    onclose: () => void;
    children: Snippet<[() => void]>;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();

  $effect(() => {
    dialog?.showModal();
  });

  function close() {
    dialog?.close();
  }

  function backdropClose(event: MouseEvent) {
    if (event.target === dialog) {
      close();
    }
  }
</script>

<dialog bind:this={dialog} class="sheet" aria-labelledby={labelledby} {onclose} onclick={backdropClose}>
  <div class="grabber" aria-hidden="true"></div>
  <div class="body">
    {@render children(close)}
  </div>
</dialog>

<style>
  .grabber {
    width: 36px;
    height: 4px;
    margin: 8px auto 0;
    border-radius: 2px;
    background: var(--surface-strong);
  }

  .body {
    padding: 4px max(20px, env(safe-area-inset-right)) max(24px, env(safe-area-inset-bottom))
      max(20px, env(safe-area-inset-left));
  }
</style>
