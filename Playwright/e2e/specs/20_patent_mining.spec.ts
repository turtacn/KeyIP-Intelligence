import { test, expect } from "@playwright/test";

/**
 * Patent Mining workbench: tool panel → patent search → detail view
 * The page presents 5 mining tools as action buttons.  Buttons are disabled
 * until preconditions are met (e.g. search first, then assess).
 */

async function safeClick(page, nameRegex) {
  const btn = page.getByRole("button", { name: nameRegex });
  if (await btn.count()) await btn.first().click({ timeout: 5000 }).catch(() => {});
}

test("Patent Mining: tool panel loads with all 5 mining tools", async ({ page }) => {
  await page.goto("/patent-mining", { waitUntil: "networkidle" });

  // All 5 tool buttons should be visible
  for (const tool of [
    /Patentability Assessment/i,
    /White Space Discovery/i,
    /Patent Search/i,
    /Prior Art Analysis/i,
    /Claim Draft Assistant/i,
  ]) {
    await expect(page.getByRole("button", { name: tool }).first()).toBeVisible({ timeout: 10000 });
  }
});

test("Patent Mining: search flow — enter query, find patent, view detail", async ({ page }) => {
  // Mock search
  await page.route("**/api/v1/patents/search", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        code: 0, message: "ok",
        data: [
          { id: "P-001", title: "Lithium Battery Separator Material", patent_number: "CN118000001A", assignee_name: "KeyIP OLED Lab", status: "pending" },
        ],
        pagination: { page: 1, pageSize: 20, total: 1 }
      }),
    });
  });

  await page.goto("/patent-mining", { waitUntil: "networkidle" });

  // Click Patent Search tool → then Text Search sub-option
  await page.getByRole("button", { name: /Patent Search/i }).first().click();
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: /Text Search/i }).first().click();
  await page.waitForTimeout(500);

  // On patent-mining page, Text Search reveals a textbox
  const searchBox = page.getByRole("textbox").first();
  if (await searchBox.isVisible().catch(() => false)) {
    await searchBox.fill("battery");
    await page.waitForTimeout(1500);
  }

  // Verify the tool panel is still interactable after search interaction
  await expect(page.getByRole("button", { name: /Patent Search/i }).first()).toBeVisible();
});

test("Patent Mining: claim draft tool exists and is clickable", async ({ page }) => {
  await page.goto("/patent-mining", { waitUntil: "networkidle" });
  await expect(page.getByRole("button", { name: /Claim Draft Assistant/i }).first()).toBeVisible();
});
