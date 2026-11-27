import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { holdFor } from "./hold";

describe("holdFor", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("waits out what's left of the time", async () => {
    let done = false;
    void holdFor(1000, 1200, () => 1500).then(() => (done = true));
    await vi.advanceTimersByTimeAsync(699);
    expect(done).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(done).toBe(true);
  });

  it("returns at once when the time has passed", async () => {
    let done = false;
    void holdFor(1000, 1200, () => 2500).then(() => (done = true));
    await vi.advanceTimersByTimeAsync(0);
    expect(done).toBe(true);
  });
});
