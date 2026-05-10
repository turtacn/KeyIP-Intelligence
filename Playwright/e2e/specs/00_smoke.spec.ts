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


test("Smoke: sidebar routes are reachable", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/dashboard/metrics", {
    totalPatents: 1234, activeAlerts: 12, portfolioValue: 987654
  });

  await mockJson(page, "**/api/openapi/v1/alerts**", {
    items: [
      { id: "A-001", title: "Potential infringement risk", riskLevel: "HIGH" },
      { id: "A-002", title: "New competitor filing", riskLevel: "MEDIUM" }
    ],
    page: 1, pageSize: 10, total: 2
  });

  await mockJson(page, "**/api/openapi/v1/portfolio/summary", {
    totalAssets: 42, totalValue: 987654, budget: 250000
  });
  await mockJson(page, "**/api/openapi/v1/portfolio/scores", {
    items: [
      { id: "P-001", score: 88 },
      { id: "P-002", score: 72 }
    ]
  });
  await mockJson(page, "**/api/openapi/v1/portfolio/coverage", {
    coverage: [
      { area: "Battery", value: 0.72 },
      { area: "AI", value: 0.55 }
    ]
  });

  await mockJson(page, "**/api/openapi/v1/lifecycle/events**", {
    items: [
      { id: "E-001", jurisdiction: "US", type: "Annuity", dueDate: "2026-03-10", urgency: "DUE_30D" },
      { id: "E-002", jurisdiction: "EP", type: "Deadline", dueDate: "2026-02-20", urgency: "OVERDUE" }
    ],
    total: 2
  });

  await mockJson(page, "**/api/openapi/v1/partners", {
    items: [
      { id: "PT-001", name: "Demo Agency", type: "Agency" },
      { id: "PT-002", name: "Demo Counsel", type: "Counsel" }
    ],
    total: 2
  });

  await page.goto("/");
  await expect(page).toHaveURL(/\/dashboard/);

  await clickSidebar(page, "Dashboard");
  await expect(page).toHaveURL(/\/dashboard/);

  await clickSidebar(page, "Patent Mining");
  await expect(page).toHaveURL(/\/patent-mining/);

  await clickSidebar(page, "Infringement Watch");
  await expect(page).toHaveURL(/\/infringement-watch/);

  await clickSidebar(page, "Portfolio");
  await expect(page).toHaveURL(/\/portfolio/);

  await clickSidebar(page, "Lifecycle");
  await expect(page).toHaveURL(/\/lifecycle/);

  await clickSidebar(page, "Partner");
  await expect(page).toHaveURL(/\/partners/);
});
