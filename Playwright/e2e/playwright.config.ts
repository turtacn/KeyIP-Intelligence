import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  timeout: 90_000,
  expect: { timeout: 10_000 },
  retries: 0,
  fullyParallel: false,
  reporter: [
    ["list"],
    ["html", { outputFolder: "../artifacts/playwright-report", open: "never" }],
  ],
  outputDir: "../artifacts/test-output",
  use: {
    baseURL: process.env.BASE_URL || "http://192.168.99.100",
    headless: true,
    locale: "en-US",
    viewport: { width: 1280, height: 720 },
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    video: "off",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    launchOptions: {
      executablePath: process.env.CHROMIUM_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
      args: [
        "--disable-dev-shm-usage",
        // CDP debug port is off by default to avoid port conflicts with
        // parallel workers.  Enable per-run with:
        //   CDP_PORT=9222 npx playwright test …
        ...(process.env.CDP_PORT ? [`--remote-debugging-port=${process.env.CDP_PORT}`, "--remote-debugging-address=0.0.0.0"] : []),
      ],
    },
  },
});
