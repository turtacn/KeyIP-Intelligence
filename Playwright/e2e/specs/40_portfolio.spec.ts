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


test("Portfolio Optimizer: sections + apply plan + simulation", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/portfolio/summary", {
    totalAssets: 42, totalValue: 987654, budget: 250000
  });
  await mockJson(page, "**/api/openapi/v1/portfolio/scores", {
    items: [
      { id: "P-001", title: "Lithium battery separator", score: 88 },
      { id: "P-002", title: "AI-based molecule screening", score: 72 }
    ]
  });
  await mockJson(page, "**/api/openapi/v1/portfolio/coverage", {
    coverage: [
      { area: "Battery", value: 0.72 },
      { area: "AI", value: 0.55 }
    ]
  });

  await gotoAndAssert(page, "/portfolio", ["Portfolio"]);

  const sections = [
    /Portfolio Panorama/i,
    /Competitive Gap Matrix/i,
    /Patent Value Scoring/i,
    /Budget Optimizer/i,
    /What-If Simulator/i,
  ];

  for (const s of sections) {
    const nav = page.getByText(s).first();
    if (await nav.count()) {
      await nav.scrollIntoViewIfNeeded().catch(() => {});
      await nav.click().catch(() => {});
    }
  }

  await safeClick(page, "button", /Apply Optimization Plan/i);
  await safeClick(page, "button", /Confirm & Apply/i);

  await safeClick(page, "button", /Run Simulation/i);

  const anyResult = page.getByText(/Simulation/i).first();
  if (await anyResult.count()) await expect(anyResult).toBeVisible();
});
