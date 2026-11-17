// How a song's lyrics source is named in the app.
export function sourceLabel(code: string): string {
  switch (code) {
    case "user":
      return "your lyrics";
    case "lrclib":
      return "LRCLIB";
    case "netease":
      return "NetEase Cloud Music";
    case "qqmusic":
      return "QQ Music";
    default:
      return code;
  }
}

// A song length as m:ss. Empty when the length isn't known.
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }
  const total = Math.round(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}
