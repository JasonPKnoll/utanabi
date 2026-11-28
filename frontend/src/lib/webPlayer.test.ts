import { describe, expect, it } from "vitest";

import { connectPlayer, type Sdk, type SdkPlayer, type WebPlayerState } from "./webPlayer.svelte";

// A Spotify SDK whose player says what each test makes it say.
function fakeSdk(connects = true) {
  const listeners: Record<string, (data: { device_id?: string; message?: string }) => void> = {};
  let asked: ((give: (token: string) => void) => void) | undefined;
  let name = "";
  class Player implements SdkPlayer {
    constructor(options: { name: string; getOAuthToken: (give: (token: string) => void) => void }) {
      name = options.name;
      asked = options.getOAuthToken;
    }
    addListener(event: string, listener: (data: { device_id?: string }) => void) {
      listeners[event] = listener;
      return true;
    }
    connect() {
      return Promise.resolve(connects);
    }
    activateElement() {
      return Promise.resolve();
    }
  }
  return {
    sdk: { Player } as unknown as Sdk,
    say: (event: string, data: { device_id?: string; message?: string } = {}) => listeners[event]?.(data),
    token: () => new Promise<string>((done) => asked?.(done)),
    name: () => name,
  };
}

function started(): WebPlayerState {
  return { status: "starting", deviceId: "" };
}

describe("connectPlayer", () => {
  it("is ready with its device once Spotify lists it", () => {
    const fake = fakeSdk();
    const state = started();
    connectPlayer(fake.sdk, () => Promise.resolve("t"), state);
    fake.say("ready", { device_id: "abc123" });
    expect(state).toEqual({ status: "ready", deviceId: "abc123" });
    expect(fake.name()).toBe("Utanabi");
  });

  it("hands the player the app's access token", async () => {
    const fake = fakeSdk();
    connectPlayer(fake.sdk, () => Promise.resolve("access-token"), started());
    expect(await fake.token()).toBe("access-token");
  });

  it("is starting again while offline", () => {
    const fake = fakeSdk();
    const state = started();
    connectPlayer(fake.sdk, () => Promise.resolve("t"), state);
    fake.say("ready", { device_id: "abc123" });
    fake.say("not_ready", { device_id: "abc123" });
    expect(state).toEqual({ status: "starting", deviceId: "" });
  });

  it.each([
    ["initialization_error", "unavailable"],
    ["account_error", "unavailable"],
    ["authentication_error", "reconnect"],
  ])("after %s is %s", (event, want) => {
    const fake = fakeSdk();
    const state = started();
    connectPlayer(fake.sdk, () => Promise.resolve("t"), state);
    fake.say(event, { message: "no" });
    expect(state.status).toBe(want);
  });

  it("is unavailable when it can't connect", async () => {
    const fake = fakeSdk(false);
    const state = started();
    connectPlayer(fake.sdk, () => Promise.resolve("t"), state);
    await Promise.resolve();
    await Promise.resolve();
    expect(state.status).toBe("unavailable");
  });
});
