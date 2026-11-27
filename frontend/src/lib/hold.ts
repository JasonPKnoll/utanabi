// How long the song-building loader stays up, at least: one full bloom, so a fast save shows the firework for a
// moment instead of a flicker.
export const buildingAtLeastMs = 1200;

// Waits until ms have passed since started (a Date.now() time), and returns at once when they already have.
export function holdFor(started: number, ms: number, now: () => number = Date.now): Promise<void> {
  const left = started + ms - now();
  return left > 0 ? new Promise((resolve) => setTimeout(resolve, left)) : Promise.resolve();
}
