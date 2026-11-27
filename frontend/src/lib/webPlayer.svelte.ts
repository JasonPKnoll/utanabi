// The page as a Spotify player: Spotify's Web Playback SDK makes this browser tab a Spotify device, named Utanabi,
// so a song can play here without the Spotify app, which iOS suspends in the background and Spotify then stops
// listing. It needs Spotify Premium, and a connection made with the streaming permission. One player serves every
// song page, and keeps playing as you move around the app.

export type WebPlayerStatus = "off" | "starting" | "ready" | "unavailable" | "reconnect";

export interface WebPlayerState {
  // ready once Spotify lists this page as a device; unavailable when the browser or the account can't play here;
  // reconnect when the connection was made without the permission to stream.
  status: WebPlayerStatus;
  deviceId: string;
}

export const webPlayer: WebPlayerState = $state({ status: "off", deviceId: "" });

// The name Spotify shows for this page among your devices.
export const webPlayerName = "Utanabi";

const sdkSource = "https://sdk.scdn.co/spotify-player.js";

interface PlayerEvent {
  device_id?: string;
  message?: string;
}

export interface SdkPlayer {
  addListener(event: string, listener: (data: PlayerEvent) => void): boolean;
  connect(): Promise<boolean>;
  activateElement(): Promise<void>;
}

export interface Sdk {
  Player: new (options: {
    name: string;
    getOAuthToken: (give: (token: string) => void) => void;
    volume: number;
  }) => SdkPlayer;
}

let player: SdkPlayer | null = null;

// Makes a player with sdk, keeping state up with what it says: ready with its device ID, or why it can't play here.
// token gives an access token whenever the player asks for one.
export function connectPlayer(sdk: Sdk, token: () => Promise<string>, state: WebPlayerState): SdkPlayer {
  const made = new sdk.Player({
    name: webPlayerName,
    volume: 0.8,
    getOAuthToken: (give) => {
      token()
        .then(give)
        .catch(() => (state.status = "unavailable"));
    },
  });
  made.addListener("ready", ({ device_id }) => {
    state.status = "ready";
    state.deviceId = device_id ?? "";
  });
  // Gone offline for a moment, such as with the network; the player connects again by itself.
  made.addListener("not_ready", () => {
    state.status = "starting";
    state.deviceId = "";
  });
  // The browser can't play Spotify's protected audio, or the account isn't Premium.
  made.addListener("initialization_error", () => (state.status = "unavailable"));
  made.addListener("account_error", () => (state.status = "unavailable"));
  made.addListener("authentication_error", () => (state.status = "reconnect"));
  made
    .connect()
    .then((ok) => {
      if (!ok) {
        state.status = "unavailable";
      }
    })
    .catch(() => (state.status = "unavailable"));
  return made;
}

// Starts the page's player, once: loads Spotify's SDK, which calls back when it's ready.
export function startWebPlayer(token: () => Promise<string>): void {
  if (webPlayer.status !== "off") {
    return;
  }
  webPlayer.status = "starting";
  const page = window as unknown as { onSpotifyWebPlaybackSDKReady?: () => void; Spotify?: Sdk };
  page.onSpotifyWebPlaybackSDKReady = () => {
    if (page.Spotify) {
      player = connectPlayer(page.Spotify, token, webPlayer);
    } else {
      webPlayer.status = "unavailable";
    }
  };
  const script = document.createElement("script");
  script.src = sdkSource;
  script.async = true;
  script.onerror = () => (webPlayer.status = "unavailable");
  document.head.append(script);
}

// Lets the player make sound. Mobile browsers only allow that from a tap, so call it inside one, before playing.
export function activateWebPlayer(): void {
  // A browser that refuses still plays once the user taps play in the page again; there's nothing more to do here.
  player?.activateElement().catch(() => undefined);
}
