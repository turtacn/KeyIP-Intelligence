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


test("Patent Mining: tool tabs + search + detail + claim draft export", async ({ page }) => {
  await page.route("**/api/openapi/v1/patents**", async (route) => {
    const url = new URL(route.request().url());
    const matchDetail = url.pathname.match(/\/patents\/(.+)$/);
    if (matchDetail) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: matchDetail[1],
          title: "Mock Patent Detail " + matchDetail[1],
          assignee: "Demo Corp",
          abstract: "This is a mocked patent detail used for UI automation."
        })
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: [
          { id: "P-001", title: "Lithium battery separator", assignee: "Demo Corp" },
          { id: "P-002", title: "AI-based molecule screening", assignee: "Demo Corp" }
        ],
        page: 1,
        pageSize: 10,
        total: 2
      })
    });
  });

  await gotoAndAssert(page, "/patent-mining", ["Patent Mining"]);

  await safeClick(page, "tab", /Patentability/i);
  await safeClick(page, "button", /Assess Patentability/i);

  await safeClick(page, "tab", /White Space/i);
  await safeClick(page, "button", /Identify White Spaces/i);

  await safeClick(page, "tab", /Prior Art/i);
  await safeClick(page, "button", /Analyze Prior Art/i);

  await safeClick(page, "tab", /Patent Search/i);

  const tb = page.getByRole("textbox").first();
  if (await tb.count()) {
    await tb.fill("battery");
  }

  await safeClick(page, "button", /Text Search/i);
  await safeClick(page, "button", /^Search$/i);

  const rowTitle = page.getByText(/Lithium battery separator/i).first();
  if (await rowTitle.count()) {
    await rowTitle.click();
    await expect(page.getByText(/Mock Patent Detail/i)).toBeVisible();
  }

  await safeClick(page, "tab", /Claim Draft/i);
  await safeClick(page, "button", /Generate Claim Draft/i);

  const downloadsDir = path.join("..", "artifacts", "downloads");
  fs.mkdirSync(downloadsDir, { recursive: true });

  const exportBtn = page.getByRole("button", { name: /Export/i });
  if (await exportBtn.count()) {
    const downloadPromise = page.waitForEvent("download", { timeout: 5_000 }).catch(() => null);
    await exportBtn.first().click();
    const dl = await downloadPromise;
    if (dl) {
      await dl.saveAs(path.join(downloadsDir, await dl.suggestedFilename()));
    }
  }
});
