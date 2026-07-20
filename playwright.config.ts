import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  timeout: 60_000,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : 1,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: "http://127.0.0.1:5173",
    trace: "on-first-retry",
    navigationTimeout: 30000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    ...(process.env.CI
      ? [
          {
            name: "firefox",
            use: { ...devices["Desktop Firefox"] },
          },
        ]
      : []),
  ],
  webServer: [
    {
      command: "npm run dev:api",
      url: "http://127.0.0.1:3000/api/runtime",
      timeout: 180_000,
      // Always start a fresh API server for deterministic e2e env settings.
      reuseExistingServer: false,
      env: {
        ...process.env,
        APP_MODE: "dev",
        DEV_MODE: "false",
        AUTH_ENABLED: "true",
        AUTH_TOKENS: "viewer:viewer:e2e-viewer-token,operator:operator:e2e-operator-token,admin:admin:e2e-admin-token",
        WRITE_ACTIONS_ENABLED: "true",
      },
    },
    {
      command: "npm run dev:web -- --host 127.0.0.1 --port 5173",
      url: "http://127.0.0.1:5173",
      timeout: 180_000,
      reuseExistingServer: !process.env.CI,
    },
  ],
});
