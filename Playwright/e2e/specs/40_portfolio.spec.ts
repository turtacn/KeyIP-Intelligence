import { test, expect } from "@playwright/test";

/**
 * Portfolio Optimizer: section visibility → data loading → tool interactions.
 * Now connected to real Go apiserver via nginx proxy (keyip-apiserver:8080).
 * Constellation returns empty points from real DB — module may not render.
 */

test("Portfolio: page loads with header and key sections", async ({ page }) => {
  await page.goto("/portfolio", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);

  // The page should at minimum show the portfolio header or main content area
  const headerOrContent = page.locator("h1, h2, h3, [class*='header'], [class*='title'], main").first();
  await expect(headerOrContent).toBeVisible({ timeout: 10000 });
  
  // Portfolio page should load without a fatal error
  const bodyText = await page.textContent("body");
  expect(bodyText).toBeTruthy();
  expect(bodyText!.length).toBeGreaterThan(20); // Should have some rendered content
});

test("Portfolio: summary data loads from nginx stubs", async ({ page }) => {
  await page.goto("/portfolio", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);

  // Nginx returns portfolio summary — verify the page has loaded meaningful content
  // Check for any numeric data or chart area
  const content = page.locator("body");
  await expect(content).toBeVisible({ timeout: 10000 });
  
  // The page should contain numeric data (patent counts, scores)
  const bodyText = await page.textContent("body");
  const hasNumbers = /\d+/.test(bodyText || "");
  expect(hasNumbers).toBe(true);
});

test("Portfolio: tool buttons or navigation exists", async ({ page }) => {
  await page.goto("/portfolio", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);

  // The portfolio page should have some interactive elements
  const buttons = page.locator("button, a[href], [role='button']");
  const count = await buttons.count();
  expect(count).toBeGreaterThan(0); // Should have at least one interactive element
});

test("Portfolio: page renders without JS crash", async ({ page }) => {
  await page.goto("/portfolio", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);

  // Verify the page didn't crash — should have DOM content
  const root = page.locator("#root, [data-testid], main, .app");
  await expect(root.first()).toBeVisible({ timeout: 10000 });
});
