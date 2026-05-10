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
    baseURL: process.env.BASE_URL || "http://localhost:19666",
    headless: true,
    viewport: { width: 1280, height: 720 },
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    video: { mode: "on", size: { width: 1280, height: 720 } },
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    launchOptions: { args: ["--disable-dev-shm-usage"] },
  },
});
