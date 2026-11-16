import { defineConfig, devices } from "@playwright/test";

// The smoke test runs against the server-mode build. Build it first with `task build:server`.
const port = process.env.SMOKE_PORT ?? "18080";
const baseURL = `http://localhost:${port}`;

export default defineConfig({
  testDir: "e2e",
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? "list" : "line",
  use: { baseURL },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "../bin/utanabi-server",
    url: `${baseURL}/health`,
    reuseExistingServer: false,
    timeout: 30_000,
    env: {
      WAILS_SERVER_PORT: port,
      UTANABI_DATA_DIR: process.env.SMOKE_DATA_DIR ?? "../.task/smoke-data",
    },
  },
});
