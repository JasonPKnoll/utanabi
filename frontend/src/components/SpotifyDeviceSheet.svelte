<script lang="ts">
  import {
    Service as Spotify,
    type Device,
  } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
  import { choiceOf, deviceLabel, devicesToOffer } from "../lib/devices";
  import { toAppError } from "../lib/errors";
  import Sheet from "./Sheet.svelte";

  // Where Spotify plays: your devices with Spotify open, and this page's own player. Choosing one moves the song there
  // when it's playing, and is remembered on this device for next time.
  let {
    hereId,
    chosen,
    playingOn,
    paused = false,
    onchoose,
    switchTo = "",
    onswitch,
    onclose,
  }: {
    // This page's own player's device ID, or empty when it isn't ready.
    hereId: string;
    // The remembered choice, the name of the device the song is on now, and whether it's paused there.
    chosen: string;
    playingOn: string;
    paused?: boolean;
    onchoose: (device: Device) => void;
    // The other player the song can play along with, such as "YouTube", and switching to it; none when there's none.
    switchTo?: string;
    onswitch?: () => void;
    onclose: () => void;
  } = $props();

  let devices = $state<Device[] | null>(null);
  let note = $state("");

  const kinds: Record<string, string> = {
    smartphone: "Phone",
    computer: "Computer",
    tablet: "Tablet",
    speaker: "Speaker",
    tv: "TV",
    castvideo: "Cast",
    castaudio: "Cast",
    automobile: "Car",
  };

  async function look() {
    note = "";
    try {
      devices = devicesToOffer((await Spotify.Devices()) ?? [], hereId);
    } catch (err) {
      devices = [];
      note = toAppError(err).message;
    }
  }
  void look();
</script>

<Sheet labelledby="devices-title" {onclose}>
  {#snippet children(close)}
    <header>
      <h2 id="devices-title">Play on</h2>
      <button type="button" class="icon-button" aria-label="Close" onclick={close}>
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          aria-hidden="true"><line x1="6" y1="6" x2="18" y2="18" /><line x1="18" y1="6" x2="6" y2="18" /></svg
        >
      </button>
    </header>

    {#if devices === null}
      <p class="muted">Looking for your Spotify…</p>
    {:else}
      <ul class="devices">
        {#each devices as device (device.id)}
          {@const label = deviceLabel(device, hereId)}
          {@const picked = choiceOf(device, hereId) === chosen}
          <li>
            <button
              type="button"
              class="device"
              class:picked
              aria-pressed={picked}
              onclick={() => {
                onchoose(device);
                close();
              }}
            >
              <span class="name">{label}</span>
              <span class="kind">
                {kinds[device.type.toLowerCase()] ??
                  device.type}{#if device.name === playingOn && device.id !== hereId}&nbsp;·&nbsp;{paused
                    ? "paused"
                    : "playing"}{/if}
              </span>
            </button>
          </li>
        {/each}
      </ul>
      {#if devices.length === 0 && !note}
        <p class="muted">No Spotify is open. Open the Spotify app on your phone or computer.</p>
      {/if}
      <p class="muted small">Don't see your phone? Open Spotify on it, then look again.</p>
      <button type="button" class="button again" onclick={look}>Look again</button>
    {/if}
    {#if note}<p class="alert" role="alert">{note}</p>{/if}
    {#if switchTo && onswitch}
      <button
        type="button"
        class="button again"
        onclick={() => {
          close();
          onswitch();
        }}>Play along with {switchTo} instead</button
      >
    {/if}
  {/snippet}
</Sheet>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0 4px;
  }

  h2 {
    margin: 0;
    font-size: 1.125rem;
    font-weight: 650;
  }

  p {
    margin: 10px 4px 0;
  }

  .small {
    font-size: 0.875rem;
  }

  .devices {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 8px 0 0;
    padding: 0;
    list-style: none;
  }

  .device {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    width: 100%;
    min-height: 56px;
    padding: 8px 14px;
    border: 1px solid transparent;
    border-radius: 14px;
    background: var(--bg);
    color: var(--fg);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .device.picked {
    border-color: var(--link);
  }

  .name {
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .kind {
    font-size: 0.875rem;
    color: var(--muted);
  }

  .again {
    width: 100%;
    margin-top: 12px;
  }
</style>
