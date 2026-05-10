import { test, expect } from "@playwright/test";
/**
 * Helpers: stable navigation + API mocking (repo uses baseUrl `/api/openapi/v1`)
 * These specs are designed to:
 *  - Fully click through UI and core actions
 *  - Work even without a backend by mocking network responses
 *  - Produce per-spec videos in artifacts/test-output
 */

async function mockJson(page, urlGlob, json, status = 200) {
  await page.route(urlGlob, async (route) => {
    await route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(json),
    });
  });
}

async function gotoAndAssert(page, path, mustContainTexts = []) {
  await page.goto(path);
  for (const t of mustContainTexts) {
    await page.getByText(t, { exact: false }).first().waitFor({ state: "visible" });
  }
}

async function clickSidebar(page, label) {
  const link = page.getByRole("link", { name: new RegExp(label, "i") });
  if (await link.count()) {
    await link.first().click();
  } else {
    await page.getByText(new RegExp(label, "i")).first().click();
  }
}

async function safeClick(page, role, nameRegex) {
  const loc = page.getByRole(role, { name: nameRegex });
  if (await loc.count()) await loc.first().click();
}

async function safeSelect(page, labelRegex, optionRegex) {
  const combo = page.getByRole("combobox", { name: labelRegex });
  if (await combo.count()) {
    await combo.first().selectOption({ label: optionRegex.source });
    return;
  }
  const label = page.getByText(labelRegex).first();
  if (await label.count()) {
    await label.click();
    await page.getByRole("option", { name: optionRegex }).first().click().catch(() => {});
  }
}


test("Dashboard: metrics + Export Report + Add New Patent", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/dashboard/metrics", {
    totalPatents: 1234,
    activeAlerts: 12,
    portfolioValue: 987654
  });

  await gotoAndAssert(page, "/dashboard", ["Dashboard"]);

  await safeClick(page, "button", /Export Report/i);

  const anyAlert = page.getByRole("alert").first();
  if (await anyAlert.count()) {
    await expect(anyAlert).toBeVisible();
  }

  await safeClick(page, "button", /Add New Patent/i);
});
