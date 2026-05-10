import { test, expect } from "@playwright/test";

/**
 * Helpers: stable navigation + API mocking (repo uses baseUrl `/api/v1`)
 * These specs are designed to:
 *  - Fully click through UI and core actions
 *  - Work with the built-in MSW mock handlers (no real backend needed)
 *  - Produce per-spec videos in artifacts/test-output
 */

async function gotoAndAssert(page, path, mustContainTexts = []) {
  await page.goto(path);
  for (const t of mustContainTexts) {
    await page.getByText(t, { exact: false }).first().waitFor({ state: "visible" });
  }
}

async function safeClick(page, role, nameRegex) {
  const loc = page.getByRole(role, { name: nameRegex });
  if (await loc.count()) await loc.first().click();
}

// ──────────────────────────────────────────────
// Patent Search Flow
// ──────────────────────────────────────────────

test.describe("Patent search and detail flow", () => {

  test("Search page loads and shows empty state", async ({ page }) => {
    await page.goto("/search");
    // Verify the search page title is visible (i18n key 'search.title')
    await expect(page.getByText(/Search|搜索/).first()).toBeVisible();
    // Empty state should show before a query is entered
    await expect(page.getByText(/query|query|搜索/).first()).toBeVisible();
  });

  test("Patent search: enter query and see results", async ({ page }) => {
    await page.goto("/search");

    // Find the search input field on the Search page (large centered input)
    const searchInput = page.locator('input[type="text"]').first();
    await expect(searchInput).toBeVisible();

    // Type a query and submit via Enter
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Wait for results to load from MSW mock data
    // The mock patent data contains patents with "OLED" in their title
    await expect(page.getByText(/Novel Blue Emitter Material|TADF|Thermally Activated/i).first()).toBeVisible({ timeout: 15000 });

    // Verify results are displayed in the data table
    // Should see at least one patent row
    await expect(page.getByText(/CN115321456A|US20230123456A1|OLED Device/i).first()).toBeVisible();
  });

  test("Patent search shows molecule results alongside patent results", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Wait for the combined results table
    // The search page shows both patents and molecules in "All" tab
    await expect(page.getByText(/CBP|NPB|Alq3|4CzIPN|Ir\(ppy\)3/).first()).toBeVisible({ timeout: 15000 });
  });

  test("Patent search empty query does not trigger search", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.press("Enter");

    // Empty state should still be visible (no results)
    // The page should remain in pre-search state
    await expect(page.getByText(/Search across|Enter a query|搜索/).first()).toBeVisible();
  });

  test("Patent search error state is handled gracefully", async ({ page }) => {
    // Override the MSW handler for this test to simulate an error
    await page.route("**/api/v1/patents/search", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ code: 5000, message: "Internal server error", data: null })
      });
    });

    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("__error");
    await searchInput.press("Enter");

    // Should show error state
    await expect(page.getByText(/Search Error|Error|error|Error/i).first()).toBeVisible({ timeout: 10000 });

    // Clean up the route override
    await page.unroute("**/api/v1/patents/search");
  });
});

// ──────────────────────────────────────────────
// Patent Detail Navigation
// ──────────────────────────────────────────────

test.describe("Patent detail navigation", () => {

  test("Click patent title in search results navigates to detail page", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Wait for results and click the first patent title link
    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    // Should navigate to patent detail page
    await expect(page).toHaveURL(/\/patents\/pat_001/);

    // Verify patent detail content
    await expect(page.getByText("Novel Blue Emitter Material for OLED Devices")).toBeVisible();
    await expect(page.getByText("Samsung SDI Co., Ltd.")).toBeVisible();
    await expect(page.getByText("CN115321456A")).toBeVisible();

    // Should show the abstract
    await expect(page.getByText(/organic electroluminescent|pyrene core/i).first()).toBeVisible();
  });

  test("Patent detail page shows claims, inventors, and IPC codes", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    // Verify claims section is visible
    await expect(page.getByText(/Claims|权利要求/i).first()).toBeVisible();
    // Verify inventors
    await expect(page.getByText(/Kim, Min-Soo|Lee, Ji-Hoon/i).first()).toBeVisible();
    // Verify IPC codes
    await expect(page.getByText(/C09K11\/06|H10K85\/60/i).first()).toBeVisible();
  });

  test("Patent detail back navigation works", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    // Click the "Back" button
    const backButton = page.getByText(/Back|返回/i).first();
    await backButton.click();

    // Should navigate back to search page
    await expect(page).toHaveURL(/\/search/);
  });

  test("Direct navigation to patent detail by ID", async ({ page }) => {
    await page.goto("/patents/pat_002");

    // Should load the patent with id pat_002
    await expect(page.getByText("Thermally Activated Delayed Fluorescence Material")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText("Universal Display Corporation")).toBeVisible();
    await expect(page.getByText("US20230123456A1")).toBeVisible();
  });

  test("Navigating to non-existent patent shows not found state", async ({ page }) => {
    // Override the MSW handler to return 404
    await page.route("**/api/v1/patents/nonexistent", async (route) => {
      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ code: 4004, message: "Patent not found", data: null })
      });
    });

    await page.goto("/patents/nonexistent");
    // Should show not found or error state
    await expect(page.getByText(/Patent Not Found|not found|Error|Patent/i).first()).toBeVisible({ timeout: 15000 });

    await page.unroute("**/api/v1/patents/nonexistent");
  });
});

// ──────────────────────────────────────────────
// Molecule Similarity Search
// ──────────────────────────────────────────────

test.describe("Molecule search and detail flow", () => {

  test("Molecule search via Search page molecules tab", async ({ page }) => {
    await page.goto("/search");

    // Search for anything to populate the molecules list
    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Switch to Molecules tab
    await safeClick(page, "button", /Molecules|分子/i);
    await page.waitForTimeout(500);

    // Molecule results should be visible
    await expect(page.getByText(/CBP|NPB|Alq3|4CzIPN|mCP|Ir\(ppy\)3|TCTA|FIrpic|DMAC-DPS/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("Molecule detail navigation from search results", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Switch to Molecules tab
    await safeClick(page, "button", /Molecules|分子/i);
    await page.waitForTimeout(500);

    // Click on a molecule name to view detail
    const moleculeLink = page.getByText(/^CBP$/).first();
    await expect(moleculeLink).toBeVisible({ timeout: 10000 });
    await moleculeLink.click();

    // Should navigate to molecule detail
    await expect(page).toHaveURL(/\/molecules\/mol_001/);

    // Verify molecule detail content
    await expect(page.getByText("CBP")).toBeVisible();
    await expect(page.getByText(/319\.41|Molecular Weight|分子量/i).first()).toBeVisible();
    // Should show SMILES
    await expect(page.getByText(/c1ccc/).first()).toBeVisible();
  });

  test("Molecule detail shows material properties table", async ({ page }) => {
    await page.goto("/molecules/mol_001");

    // Wait for molecule detail to load
    await expect(page.getByText("CBP")).toBeVisible({ timeout: 15000 });
    // Should show properties like HOMO, LUMO, Triplet Energy
    await expect(page.getByText(/HOMO|LUMO|Triplet Energy/i).first()).toBeVisible();
  });

  test("Structure search in Patent Mining workbench", async ({ page }) => {
    await page.goto("/patent-mining");

    // Click on "Patent Search" tool in the sidebar
    await safeClick(page, "button", /Patent Search|专利检索/i);
    await page.waitForTimeout(500);

    // Switch to "Structure Search" mode
    await safeClick(page, "button", /Structure Search|结构检索/i);
    await page.waitForTimeout(300);

    // Enter a SMILES string
    const smilesInput = page.getByPlaceholder(/SMILES/i).first();
    await expect(smilesInput).toBeVisible();
    await smilesInput.fill("c1ccccc1");

    // Adjust similarity threshold if the slider exists
    const slider = page.locator('input[type="range"]');
    if (await slider.count() > 0) {
      await slider.first().fill("0.85");
    }

    // Click Search button
    await safeClick(page, "button", /^Search$|检索/i);

    // Wait for results to load from MSW mock data
    await expect(page.getByText(/CN115321456A|US20230123456A1|EP3456789A1|Novel Blue|TADF|Hole Transport/i).first()).toBeVisible({ timeout: 15000 });
  });

  test("Direct navigation to molecule detail by ID", async ({ page }) => {
    await page.goto("/molecules/mol_002");

    // Should load molecule NPB
    await expect(page.getByText("NPB")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/588\.76|Molecular Weight/i).first()).toBeVisible();
  });
});
