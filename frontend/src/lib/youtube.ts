// YouTube's embedded player (the IFrame Player API): a song's video plays in the page, and the page reads how far in
// it is whenever it likes, so the lyrics follow it exactly. The frame comes from youtube-nocookie.com, which keeps
// YouTube's cookies off until the video plays.

export interface YouTubePlayer {
  playVideo(): void;
  pauseVideo(): void;
  seekTo(seconds: number, allowSeekAhead: boolean): void;
  getCurrentTime(): number;
  getPlayerState(): number;
  destroy(): void;
}

interface PlayerEvent {
  data: number;
}

export interface YouTubeApi {
  Player: new (
    frame: HTMLIFrameElement,
    options: {
      events: {
        onReady?: () => void;
        onStateChange?: (event: PlayerEvent) => void;
        onError?: (event: PlayerEvent) => void;
      };
    },
  ) => YouTubePlayer;
}

// The player's states, as getPlayerState and onStateChange give them.
export const unstarted = -1;
export const ended = 0;
export const playingState = 1;
export const pausedState = 2;
export const buffering = 3;
export const cued = 5;

const apiSource = "https://www.youtube.com/iframe_api";

let loading: Promise<YouTubeApi> | null = null;

// Loads YouTube's player API, once.
export function loadYouTube(): Promise<YouTubeApi> {
  if (loading) {
    return loading;
  }
  loading = new Promise((resolve, reject) => {
    const page = window as unknown as { YT?: YouTubeApi; onYouTubeIframeAPIReady?: () => void };
    if (page.YT?.Player) {
      resolve(page.YT);
      return;
    }
    page.onYouTubeIframeAPIReady = () => (page.YT ? resolve(page.YT) : reject(new Error("youtube: no player API")));
    const script = document.createElement("script");
    script.src = apiSource;
    script.async = true;
    script.onerror = () => {
      loading = null;
      reject(new Error("youtube: player API didn't load"));
    };
    document.head.append(script);
  });
  return loading;
}

// The address of a video's embedded player, for this page at origin: it plays inline on a phone instead of going
// full screen, and its controls can be driven from the page.
export function embedURL(videoId: string, origin: string): string {
  const query = new URLSearchParams({ enablejsapi: "1", playsinline: "1", rel: "0", origin });
  return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoId)}?${query.toString()}`;
}

// What a player error means for you: YouTube's codes 100 (gone), 101 and 150 (its owner doesn't allow embedding), 2
// (a bad ID), 5 (can't play in this browser), and 153 (the page's address didn't reach YouTube).
export function errorNote(code: number): string {
  switch (code) {
    case 101:
    case 150:
      return "This video's owner doesn't let it play outside YouTube. Pick another.";
    case 100:
      return "This video is gone or private. Pick another.";
    case 5:
      return "This video can't play in this browser.";
    default:
      return "This video couldn't play. Pick another.";
  }
}
