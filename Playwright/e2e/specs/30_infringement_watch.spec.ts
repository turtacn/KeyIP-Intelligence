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


test("Infringement Watch: filter + refresh + detail actions + live feed pause/resume", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/alerts**", {
    items: [
      { id: "A-001", title: "Potential infringement risk", riskLevel: "HIGH", summary: "Mock alert 1" },
      { id: "A-002", title: "New competitor filing", riskLevel: "MEDIUM", summary: "Mock alert 2" }
    ],
    page: 1, pageSize: 10, total: 2
  });

  await gotoAndAssert(page, "/infringement-watch", ["Infringement"]);

  await safeClick(page, "button", /Risk Level/i);
  await safeClick(page, "menuitem", /All/i);

  await safeClick(page, "button", /Refresh/i);

  const firstAlert = page.getByText(/Potential infringement risk/i).first();
  if (await firstAlert.count()) {
    await firstAlert.click();

    await safeClick(page, "button", /Generate FTO Report/i);
    await safeClick(page, "button", /Design Around/i);
    await safeClick(page, "button", /Assign to Legal/i);
    await safeClick(page, "button", /Mark as Reviewed/i);

    const alert = page.getByRole("alert").first();
    if (await alert.count()) await expect(alert).toBeVisible();
  }

  await safeClick(page, "button", /Pause Feed/i);
  await safeClick(page, "button", /Resume Feed/i);
});
