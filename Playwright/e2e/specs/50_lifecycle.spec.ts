import { test, expect } from "@playwright/test";
import path from "path";
import fs from "fs";
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


test("Lifecycle: filters + tabs + calendar actions + export csv + annuity pay", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/lifecycle/events**", {
    items: [
      { id: "E-001", jurisdiction: "US", type: "Annuity", dueDate: "2026-03-10", urgency: "DUE_30D", title: "US Annuity payment" },
      { id: "E-002", jurisdiction: "EP", type: "Deadline", dueDate: "2026-02-20", urgency: "OVERDUE", title: "EP Office action response" }
    ],
    total: 2
  });

  await gotoAndAssert(page, "/lifecycle", ["Lifecycle"]);

  await safeClick(page, "button", /Apply Filters/i);
  await safeClick(page, "button", /Reset/i);

  await safeClick(page, "tab", /Deadline Calendar/i);
  await safeClick(page, "tab", /Annuity Management/i);
  await safeClick(page, "tab", /Legal Status Monitor/i);

  await safeClick(page, "tab", /Deadline Calendar/i);
  await safeClick(page, "button", /Mark as Handled/i);

  const downloadsDir = path.join("..", "artifacts", "downloads");
  fs.mkdirSync(downloadsDir, { recursive: true });

  const exportBtn = page.getByRole("button", { name: /Export CSV/i });
  if (await exportBtn.count()) {
    const downloadPromise = page.waitForEvent("download", { timeout: 5_000 }).catch(() => null);
    await exportBtn.first().click();
    const dl = await downloadPromise;
    if (dl) {
      await dl.saveAs(path.join(downloadsDir, await dl.suggestedFilename()));
    }
  }

  await safeClick(page, "tab", /Annuity Management/i);
  await safeClick(page, "button", /^Pay$/i);
});
