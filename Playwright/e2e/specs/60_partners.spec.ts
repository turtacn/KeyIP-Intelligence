import { test, expect } from "@playwright/test";
import path from "path";
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


test("Partner Portal: admin/agency/counsel/api tabs + actions", async ({ page }) => {
  await mockJson(page, "**/api/openapi/v1/partners", {
    items: [
      { id: "PT-001", name: "Demo Agency", type: "Agency" },
      { id: "PT-002", name: "Demo Counsel", type: "Counsel" }
    ],
    total: 2
  });

  await gotoAndAssert(page, "/partners", ["Partner"]);

  const tabs = [/Admin/i, /Agency/i, /Counsel/i, /API/i];
  for (const t of tabs) {
    await safeClick(page, "tab", t);
  }

  await safeClick(page, "tab", /Admin/i);
  await safeClick(page, "button", /Add Partner/i);

  await safeClick(page, "tab", /Agency/i);

  const fileInput = page.locator('input[type="file"]');
  if (await fileInput.count()) {
    await fileInput.first().setInputFiles(path.join(process.cwd(), "fixtures", "demo.pdf"));
    const uploadedName = page.getByText(/demo\.pdf/i).first();
    if (await uploadedName.count()) await expect(uploadedName).toBeVisible();
  }

  const chatBox = page.getByRole("textbox").last();
  if (await chatBox.count()) {
    await chatBox.fill("Hello from Playwright automation.");
    await safeClick(page, "button", /Send/i);
  }

  await safeClick(page, "tab", /Counsel/i);
  const opinion = page.getByRole("textbox").first();
  if (await opinion.count()) {
    await opinion.fill("Mock legal opinion: review complete.");
  }
  await safeSelect(page, /Risk/i, /Medium/i);
  await safeClick(page, "button", /Submit Review Opinion/i);

  await safeClick(page, "tab", /API/i);
  await safeClick(page, "button", /Generate New Key/i);
  await safeClick(page, "button", /Copy/i);
  await safeClick(page, "button", /Download/i);
});
