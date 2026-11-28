// The app's look, as two choices kept on this device: the palette (Utanabi, Tokyo Night, and so on, in themes.css) and
// the mode (Auto follows the device's light or dark setting; Light and Dark override it). Both are applied as
// attributes on <html>, data-palette and data-theme, which the CSS reads.
export type Mode = "auto" | "light" | "dark";

export const modes: { value: Mode; label: string }[] = [
  { value: "auto", label: "Auto" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

export type Palette = "utanabi" | "tokyo-night" | "nord" | "catppuccin" | "solarized" | "vivid";

export const palettes: { value: Palette; label: string }[] = [
  { value: "utanabi", label: "Utanabi" },
  { value: "tokyo-night", label: "Tokyo Night" },
  { value: "nord", label: "Nord" },
  { value: "catppuccin", label: "Catppuccin" },
  { value: "solarized", label: "Solarized" },
  { value: "vivid", label: "Vivid" },
];

const modeKey = "utanabi.theme";
const paletteKey = "utanabi.palette";

type Store = Pick<Storage, "getItem" | "setItem">;
const local = () => window.localStorage;

// The mode chosen on this device, or Auto. Storage can be missing or refuse access, so every access is guarded.
export function loadMode(store: () => Store = local): Mode {
  const saved = read(store, modeKey);
  return saved === "light" || saved === "dark" ? saved : "auto";
}

// The palette chosen on this device, or Utanabi.
export function loadPalette(store: () => Store = local): Palette {
  const saved = read(store, paletteKey);
  return palettes.find((p) => p.value === saved)?.value ?? "utanabi";
}

// Remembers the mode on this device. It returns false when storage refuses, and the mode then lasts only until the
// app closes.
export function saveMode(mode: Mode, store: () => Store = local): boolean {
  return write(store, modeKey, mode);
}

// Remembers the palette on this device, like saveMode.
export function savePalette(palette: Palette, store: () => Store = local): boolean {
  return write(store, paletteKey, palette);
}

// Shows the palette and mode: sets the attributes the CSS reads, and colours the browser's bars to match the page.
export function applyLook(palette: Palette, mode: Mode, doc: Document = document): void {
  const root = doc.documentElement;
  if (palette === "utanabi") {
    delete root.dataset.palette;
  } else {
    root.dataset.palette = palette;
  }
  if (mode === "auto") {
    delete root.dataset.theme;
  } else {
    root.dataset.theme = mode;
  }
  // The palette's own page colours, read back from themes.css so they're written in one place.
  const style = getComputedStyle(root);
  const background = { light: style.getPropertyValue("--l-bg").trim(), dark: style.getPropertyValue("--d-bg").trim() };
  for (const meta of doc.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    const own = meta.media.includes("light") ? background.light : background.dark;
    meta.content = mode === "auto" ? own : background[mode];
  }
}

function read(store: () => Store, key: string): string | null {
  try {
    return store().getItem(key);
  } catch {
    return null;
  }
}

function write(store: () => Store, key: string, value: string): boolean {
  try {
    store().setItem(key, value);
    return true;
  } catch {
    return false;
  }
}
