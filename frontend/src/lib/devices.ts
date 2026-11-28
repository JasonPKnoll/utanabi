// Where Spotify plays: one of your Spotify devices, such as the app on your phone or your computer, or this page's own
// player. The choice is remembered on this device, by the device's name, since Spotify can give a device a new ID.

import type { Device } from "../../bindings/github.com/JasonPKnoll/utanabi/internal/features/spotify";
import { webPlayerName } from "./webPlayer.svelte";

const storageKey = "utanabi.spotifyDevice";
// What's remembered for this page's own player, whose name every page shares.
export const herePlayer = "here";

type Store = Pick<Storage, "getItem" | "setItem">;

export function loadDevice(store: () => Store = () => window.localStorage): string {
  try {
    return store().getItem(storageKey) ?? "";
  } catch {
    return "";
  }
}

// Remembers the choice on this device. It returns false when storage refuses, and the choice then lasts only until
// the screen closes.
export function saveDevice(choice: string, store: () => Store = () => window.localStorage): boolean {
  try {
    store().setItem(storageKey, choice);
    return true;
  } catch {
    return false;
  }
}

// The name a device is remembered by: this page's player as herePlayer, any other by its name.
export function choiceOf(device: Device, hereId: string): string {
  return device.id === hereId ? herePlayer : device.name;
}

// The devices to offer: this page's player first, then the rest. Another page's player is left out: it shares this
// one's name, and playing there would play in a browser you may not be looking at.
export function devicesToOffer(devices: readonly Device[], hereId: string): Device[] {
  const here = devices.filter((d) => d.id === hereId);
  const rest = devices.filter((d) => d.id !== hereId && d.name !== webPlayerName);
  return [...here, ...rest];
}

// Where to play: the device you chose, when it's open; else, on a phone, a phone, since that's where you're listening;
// else the device Spotify last played on; else any.
export function chooseDevice(
  devices: readonly Device[],
  remembered: string,
  onPhone: boolean,
  hereId: string,
): Device | null {
  const offered = devicesToOffer(devices, hereId);
  const chosen = offered.find((d) => choiceOf(d, hereId) === remembered);
  if (remembered && chosen) {
    return chosen;
  }
  if (onPhone) {
    const phone = offered.find((d) => d.type.toLowerCase() === "smartphone");
    if (phone) {
      return phone;
    }
  }
  return offered.find((d) => d.active) ?? offered[0] ?? null;
}

// How a device is shown: this page's player as this page, the rest by their name.
export function deviceLabel(device: Device, hereId: string): string {
  return device.id === hereId ? "This page" : device.name;
}
