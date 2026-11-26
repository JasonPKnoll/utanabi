// The app's theme: Auto follows the device's light or dark setting; Light and Dark override it. The choice is kept on
// this device and applied as data-theme on <html>, which app.css reads.
export type Theme = "auto" | "light" | "dark";

export const themes: { value: Theme; label: string }[] = [
  { value: "auto", label: "Auto" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

const storageKey = "utanabi.theme";

// The page background of each theme, for the browser's own bars (meta theme-color). Keep in step with --bg.
const background = { light: "#f5f6f9", dark: "#0f1420" };

type Store = Pick<Storage, "getItem" | "setItem">;

// The theme chosen on this device, or Auto. Storage can be missing or refuse access, so every access is guarded.
export function loadTheme(store: () => Store = () => window.localStorage): Theme {
  try {
    const saved = store().getItem(storageKey);
    return saved === "light" || saved === "dark" ? saved : "auto";
  } catch {
    return "auto";
  }
}

// Remembers the theme on this device. It returns false when storage refuses, and the theme then lasts only until the
// app closes.
export function saveTheme(theme: Theme, store: () => Store = () => window.localStorage): boolean {
  try {
    store().setItem(storageKey, theme);
    return true;
  } catch {
    return false;
  }
}

// Shows the theme: sets data-theme for app.css, and colours the browser's bars to match.
export function applyTheme(theme: Theme, doc: Document = document): void {
  const root = doc.documentElement;
  if (theme === "auto") {
    delete root.dataset.theme;
  } else {
    root.dataset.theme = theme;
  }
  for (const meta of doc.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    const own = meta.media.includes("light") ? background.light : background.dark;
    meta.content = theme === "auto" ? own : background[theme];
  }
}
