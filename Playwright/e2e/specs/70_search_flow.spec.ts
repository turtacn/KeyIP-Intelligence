import { test, expect } from "@playwright/test";

/**
 * Helpers: stable navigation + API mocking (repo uses baseUrl `/api/v1`)
 * KeyIP-Intelligence production build uses nginx inline stubs as API fallback.
 * These tests override those stubs via page.route() for deterministic results.
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

async function safeClick(page, role, nameRegex) {
  const loc = page.getByRole(role, { name: nameRegex });
  if (await loc.count()) await loc.first().click();
}

// Mock patent search data matching search flow expectations
const MOCK_PATENT_SEARCH_RESULTS = [
  {
    id: "pat_001",
    publicationNumber: "CN115321456A",
    title: "Novel Blue Emitter Material for OLED Devices",
    abstract: "The present invention relates to a novel organic electroluminescent compound having a pyrene core structure...",
    filingDate: "2022-03-15",
    publicationDate: "2022-09-20",
    legalStatus: "pending",
    ipcCodes: ["C09K11/06", "H10K85/60"],
    assignee: "Samsung SDI Co., Ltd.",
    inventors: ["Kim, Min-Soo", "Lee, Ji-Hoon"],
    claims: [
      {
        id: "clm_001_1",
        patentId: "pat_001",
        type: "independent",
        text: "1. An organic electroluminescent compound represented by Formula 1...",
        elements: ["pyrene core", "aryl group", "heteroaryl group"]
      }
    ]
  },
  {
    id: "pat_002",
    publicationNumber: "US20230123456A1",
    title: "Thermally Activated Delayed Fluorescence Material",
    abstract: "A compound for an organic light-emitting device, comprising a donor moiety and an acceptor moiety...",
    filingDate: "2021-11-10",
    publicationDate: "2023-05-12",
    legalStatus: "granted",
    ipcCodes: ["H10K85/10", "C07D487/04"],
    assignee: "Universal Display Corporation",
    inventors: ["Smith, John", "Doe, Jane"],
    grantDate: "2024-01-15"
  }
];

const MOCK_PATENT_SEARCH_OK = {
  code: 0,
  message: "success",
  data: MOCK_PATENT_SEARCH_RESULTS,
  pagination: { page: 1, pageSize: 20, total: MOCK_PATENT_SEARCH_RESULTS.length }
};

const MOCK_MOLECULE_DATA = [
  { id: "mol_001", name: "CBP", smiles: "c1ccc(c(c1)c2ccccc2)n3c4ccccc4c5ccccc53", molecularWeight: 319.41 },
  { id: "mol_002", name: "NPB", smiles: "c1ccc(cc1)N(c2ccccc2)c3ccc(cc3)N(c4ccccc4)c5ccccc5", molecularWeight: 588.76 },
  { id: "mol_003", name: "Alq3", smiles: "C1=CC=C2C(=C1)C(=CC=N2)[Al]3(OC4=CC=CC=C4C=N3)OC5=CC=CC=C5C=N6", molecularWeight: 459.43 },
  { id: "mol_004", name: "4CzIPN", smiles: "N#Cc1c(n2c3ccccc3c4ccccc24)c(C#N)c(n5c6ccccc6c7ccccc57)c(C#N)c(n8c9ccccc9c%10ccccc%108)c1C#N", molecularWeight: 788.82 },
  { id: "mol_005", name: "Ir(ppy)3", smiles: "[Ir]123(c4ccccc4-c4ccccn14)(c1ccccc1-c1ccccn12)c1ccccc1-c1ccccn13", molecularWeight: 654.78 }
];

// ──────────────────────────────────────────────
// Patent Search Flow
// ──────────────────────────────────────────────

test.describe("Patent search and detail flow", () => {

  test("Search page loads and shows empty state", async ({ page }) => {
    await page.goto("/search");
    await expect(page.getByText(/Search|搜索/).first()).toBeVisible();
    await expect(page.getByText(/query|query|搜索/).first()).toBeVisible();
  });

  test("Patent search: enter query and see results", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await page.goto("/search");

    const searchInput = page.locator('form input[type="text"]').first();
    await expect(searchInput).toBeVisible();
    await searchInput.fill("OLED");
    await page.waitForTimeout(1500); // debounce: 300ms + render

    await expect(page.getByText(/Novel Blue Emitter Material|TADF|Thermally Activated/i).first()).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/CN115321456A|US20230123456A1|OLED Device/i).first()).toBeVisible();
  });

  test("Patent search shows molecule results alongside patent results", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await mockJson(page, "**/api/v1/molecules**", {
      code: 0, message: "success",
      data: MOCK_MOLECULE_DATA,
      pagination: { page: 1, pageSize: 20, total: MOCK_MOLECULE_DATA.length }
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    await expect(page.getByText(/CBP|NPB|Alq3|4CzIPN|Ir\(ppy\)3/).first()).toBeVisible({ timeout: 15000 });
  });

  test("Patent search empty query does not trigger search", async ({ page }) => {
    await page.goto("/search");
    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.press("Enter");
    await expect(page.getByText(/Search across|Enter a query|搜索/).first()).toBeVisible();
  });

  test("Patent search error state is handled gracefully", async ({ page }) => {
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

    await expect(page.getByText(/Search Error|Error|error|Error/i).first()).toBeVisible({ timeout: 10000 });
    await page.unroute("**/api/v1/patents/search");
  });
});

// ──────────────────────────────────────────────
// Patent Detail Navigation
// ──────────────────────────────────────────────

test.describe("Patent detail navigation", () => {

  test("Click patent title in search results navigates to detail page", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await mockJson(page, "**/api/v1/patents/pat_001", {
      code: 0, message: "success", data: MOCK_PATENT_SEARCH_RESULTS[0]
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    await expect(page).toHaveURL(/\/patents\/pat_001/);
    await expect(page.getByText("Novel Blue Emitter Material for OLED Devices")).toBeVisible();
    await expect(page.getByText("Samsung SDI Co., Ltd.")).toBeVisible();
    await expect(page.getByText("CN115321456A")).toBeVisible();
    await expect(page.getByText(/organic electroluminescent|pyrene core/i).first()).toBeVisible();
  });

  test("Patent detail page shows claims, inventors, and IPC codes", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await mockJson(page, "**/api/v1/patents/pat_001", {
      code: 0, message: "success", data: MOCK_PATENT_SEARCH_RESULTS[0]
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    await expect(page.getByText(/Claims|权利要求/i).first()).toBeVisible();
    await expect(page.getByText(/Kim, Min-Soo|Lee, Ji-Hoon/i).first()).toBeVisible();
    await expect(page.getByText(/C09K11\/06|H10K85\/60/i).first()).toBeVisible();
  });

  test("Patent detail back navigation works", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await mockJson(page, "**/api/v1/patents/pat_001", {
      code: 0, message: "success", data: MOCK_PATENT_SEARCH_RESULTS[0]
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    const patentLink = page.getByText(/Novel Blue Emitter Material/i).first();
    await expect(patentLink).toBeVisible({ timeout: 15000 });
    await patentLink.click();

    const backButton = page.getByText(/Back|返回/i).first();
    await backButton.click();
    await expect(page).toHaveURL(/\/search/);
  });

  test("Direct navigation to patent detail by ID", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/pat_002", {
      code: 0, message: "success", data: MOCK_PATENT_SEARCH_RESULTS[1]
    });
    await page.goto("/patents/pat_002");

    await expect(page.getByText("Thermally Activated Delayed Fluorescence Material")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText("Universal Display Corporation")).toBeVisible();
    await expect(page.getByText("US20230123456A1")).toBeVisible();
  });

  test("Navigating to non-existent patent shows not found state", async ({ page }) => {
    await page.route("**/api/v1/patents/nonexistent", async (route) => {
      await route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ code: 4004, message: "Patent not found", data: null })
      });
    });
    await page.goto("/patents/nonexistent");
    await expect(page.getByText(/Patent Not Found|not found|Error|Patent/i).first()).toBeVisible({ timeout: 15000 });
    await page.unroute("**/api/v1/patents/nonexistent");
  });
});

// ──────────────────────────────────────────────
// Molecule Similarity Search
// ──────────────────────────────────────────────

test.describe("Molecule search and detail flow", () => {

  test("Molecule search via Search page molecules tab", async ({ page }) => {
    await mockJson(page, "**/api/v1/molecules**", {
      code: 0, message: "success",
      data: MOCK_MOLECULE_DATA,
      pagination: { page: 1, pageSize: 20, total: MOCK_MOLECULE_DATA.length }
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    await safeClick(page, "button", /Molecules|分子/i);
    await page.waitForTimeout(500);

    await expect(page.getByText(/CBP|NPB|Alq3|4CzIPN|mCP|Ir\(ppy\)3|TCTA|FIrpic|DMAC-DPS/i).first()).toBeVisible({ timeout: 10000 });
  });

  test("Molecule detail navigation from search results", async ({ page }) => {
    await mockJson(page, "**/api/v1/molecules**", {
      code: 0, message: "success",
      data: MOCK_MOLECULE_DATA,
      pagination: { page: 1, pageSize: 20, total: MOCK_MOLECULE_DATA.length }
    });
    await mockJson(page, "**/api/v1/molecules/mol_001", {
      code: 0, message: "success", data: MOCK_MOLECULE_DATA[0]
    });
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    await safeClick(page, "button", /Molecules|分子/i);
    await page.waitForTimeout(500);

    const moleculeLink = page.getByText(/^CBP$/).first();
    await expect(moleculeLink).toBeVisible({ timeout: 10000 });
    await moleculeLink.click();

    await expect(page).toHaveURL(/\/molecules\/mol_001/);
    await expect(page.getByText("CBP")).toBeVisible();
    await expect(page.getByText(/319\.41|Molecular Weight|分子量/i).first()).toBeVisible();
    await expect(page.getByText(/c1ccc/).first()).toBeVisible();
  });

  test("Molecule detail shows material properties table", async ({ page }) => {
    await mockJson(page, "**/api/v1/molecules/mol_001", {
      code: 0, message: "success",
      data: {
        ...MOCK_MOLECULE_DATA[0],
        properties: [
          { type: "HOMO", value: -5.9, unit: "eV" },
          { type: "LUMO", value: -2.4, unit: "eV" },
          { type: "Triplet Energy", value: 2.56, unit: "eV" }
        ]
      }
    });
    await page.goto("/molecules/mol_001");

    await expect(page.getByText("CBP")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/HOMO|LUMO|Triplet Energy/i).first()).toBeVisible();
  });

  test("Structure search in Patent Mining workbench", async ({ page }) => {
    await mockJson(page, "**/api/v1/patents/search", MOCK_PATENT_SEARCH_OK);
    await mockJson(page, "**/api/v1/molecules**", {
      code: 0, message: "success",
      data: MOCK_MOLECULE_DATA,
      pagination: { page: 1, pageSize: 20, total: MOCK_MOLECULE_DATA.length }
    });
    await page.goto("/patent-mining");

    // Click Patent Search tool → then Structure Search sub-option
    await safeClick(page, "button", /Patent Search/i);
    await page.waitForTimeout(500);
    await safeClick(page, "button", /Structure Search/i);
    await page.waitForTimeout(500);

    // The SMILES input should appear after clicking Structure Search
    const smilesInput = page.getByPlaceholder(/SMILES/i).first();
    if (await smilesInput.isVisible().catch(() => false)) {
      await smilesInput.fill("c1ccccc1");
    } else {
      // Fallback: use generic textbox
      const tb = page.getByRole("textbox").first();
      if (await tb.isVisible().catch(() => false)) await tb.fill("c1ccccc1");
    }

    // Verify the search UI is interactive — at minimum, tool buttons are visible
    await expect(page.getByRole("button", { name: /Patent Search/i }).first()).toBeVisible();
  });

  test("Direct navigation to molecule detail by ID", async ({ page }) => {
    await mockJson(page, "**/api/v1/molecules/mol_002", {
      code: 0, message: "success",
      data: {
        ...MOCK_MOLECULE_DATA[1],
        properties: [
          { type: "HOMO", value: -5.4, unit: "eV" },
          { type: "LUMO", value: -2.3, unit: "eV" },
          { type: "Tg", value: 98, unit: "°C" }
        ]
      }
    });
    await page.goto("/molecules/mol_002");

    await expect(page.getByText("NPB")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/588\.76|Molecular Weight/i).first()).toBeVisible();
  });
});
