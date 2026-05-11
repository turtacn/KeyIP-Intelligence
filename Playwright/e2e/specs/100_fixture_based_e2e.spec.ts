import { test, expect } from "@playwright/test";

// ═══════════════════════════════════════════════════════════════════════════════
//  Helpers (follow existing codebase patterns)
// ═══════════════════════════════════════════════════════════════════════════════

/** Mock an API endpoint with JSON response via page.route. */
async function mockJson(page, urlGlob: string | RegExp, json: unknown, status = 200) {
  await page.route(urlGlob, async (route) => {
    await route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(json),
    });
  });
}

/** Navigate and assert at least one text fragment is visible. */
async function gotoAndAssert(
  page,
  path: string,
  mustContainTexts: string[] = [],
) {
  await page.goto(path);
  for (const t of mustContainTexts) {
    await page
      .getByText(t, { exact: false })
      .first()
      .waitFor({ state: "visible" });
  }
}

/** Click a role-based element if it exists. */
async function safeClick(page, role: string, nameRegex: RegExp) {
  const loc = page.getByRole(role, { name: nameRegex });
  if (await loc.count()) await loc.first().click();
}

/**
 * Find the Search page's own input (inside the <form> element),
 * not the TopBar's global search input.
 */
function getSearchPageInput(page) {
  return page.locator('form input[type="text"]').first();
}

// ═══════════════════════════════════════════════════════════════════════════════
//  Fixture Data
// ═══════════════════════════════════════════════════════════════════════════════

const FIXTURE_PATENT_CARBAZOLE = {
  id: "pat_carbazole",
  publicationNumber: "CN118765432A",
  title: "咔唑类有机电致发光材料及其制备方法",
  abstract:
    "本发明涉及一种咔唑类有机电致发光材料，该材料以咔唑为核心骨架，具有高发光效率和良好的热稳定性，适用于OLED显示器件。",
  filingDate: "2023-06-15",
  publicationDate: "2024-02-20",
  legalStatus: "pending",
  ipcCodes: ["C09K11/06", "H10K85/60"],
  assignee: "京东方科技集团股份有限公司",
  inventors: ["李明", "王华"],
  claims: [
    {
      id: "clm_carb_1",
      patentId: "pat_carbazole",
      type: "independent",
      text: "1. 一种咔唑类有机电致发光材料，其特征在于，包含咔唑核心结构和芳基取代基，所述芳基取代基连接于咔唑的氮原子上。",
      elements: ["咔唑核心", "芳基取代基", "电子传输基团"],
    },
    {
      id: "clm_carb_2",
      patentId: "pat_carbazole",
      type: "dependent",
      text: "2. 根据权利要求1所述的材料，其特征在于，所述芳基取代基为苯基、联苯基或萘基。",
      elements: ["苯基", "联苯基", "萘基"],
    },
  ],
};

const FIXTURE_MOLECULE_CBP = {
  id: "mol_001",
  name: "CBP",
  smiles: "c1ccc(c(c1)c2ccccc2)n3c4ccccc4c5ccccc53",
  inchi: "InChI=1S/C24H17N",
  molecularWeight: 319.41,
  properties: [
    { type: "HOMO", value: -5.9, unit: "eV" },
    { type: "LUMO", value: -2.4, unit: "eV" },
    { type: "Triplet Energy", value: 2.56, unit: "eV" },
  ],
};

const FIXTURE_MOLECULE_NPB = {
  id: "mol_002",
  name: "NPB",
  smiles: "c1ccc(cc1)N(c2ccccc2)c3ccc(cc3)c4ccc(cc4)N(c5ccccc5)c6ccccc6",
  molecularWeight: 588.76,
  properties: [
    { type: "HOMO", value: -5.4, unit: "eV" },
    { type: "LUMO", value: -2.3, unit: "eV" },
    { type: "Tg", value: 98, unit: "°C" },
  ],
};

const FIXTURE_DASHBOARD_METRICS = {
  totalPatents: 158,
  activePatents: 142,
  pendingPatents: 10,
  highRiskAlerts: 7,
  dueThisMonth: 7,
  portfolioHealthScore: 76,
  monthlyApplicationTrend: [
    { month: "Jan", filed: 5, granted: 2 },
    { month: "Feb", filed: 3, granted: 1 },
    { month: "Mar", filed: 7, granted: 3 },
    { month: "Apr", filed: 4, granted: 2 },
    { month: "May", filed: 6, granted: 4 },
    { month: "Jun", filed: 8, granted: 2 },
  ],
  jurisdictionBreakdown: [
    { jurisdiction: "CN", count: 65 },
    { jurisdiction: "US", count: 38 },
    { jurisdiction: "EP", count: 28 },
    { jurisdiction: "JP", count: 15 },
    { jurisdiction: "KR", count: 12 },
  ],
  competitorComparison: [
    { name: "Organization", portfolioSize: 158 },
    { name: "Samsung SDI", portfolioSize: 12500 },
    { name: "LG Chem", portfolioSize: 9800 },
    { name: "UDC", portfolioSize: 4500 },
  ],
  upcomingDeadlines: [
    {
      id: "le_001",
      patentId: "pat_001",
      jurisdiction: "CN",
      eventType: "annuity_due",
      dueDate: "2026-09-20",
      feeAmount: 1200,
      currency: "CNY",
      status: "pending",
    },
  ],
  recentAlerts: [
    {
      id: "alert_001",
      targetPatentId: "pat_001",
      triggerMoleculeId: "mol_001",
      riskLevel: "HIGH",
      literalScore: 0.88,
      docScore: 0.76,
      detectedAt: "2026-05-10T10:30:00Z",
      status: "new",
    },
  ],
};

const PATENT_SEARCH_OK = (data) => ({
  code: 0,
  message: "success",
  data,
  pagination: { page: 1, pageSize: 20, total: data.length },
});

const MOLECULE_LIST_OK = (data) => ({
  code: 0,
  message: "success",
  data,
  pagination: { page: 1, pageSize: 20, total: data.length },
});

const SINGLE_OK = (data) => ({
  code: 0,
  message: "success",
  data,
});

// ═══════════════════════════════════════════════════════════════════════════════
//  Scenario 1: FTO 分析完整流程
//  Search patent → View detail (claims, assignee) → Infringement assessment
//  Uses default MSW data (page.route does not reliably override MSW for POST)
// ═══════════════════════════════════════════════════════════════════════════════

test.describe("Scenario 1: FTO 分析完整流程", () => {
  test("搜索专利 → 查看详情 → 侵权评估", async ({ page }) => {
    // Step 1: Search for "OLED" (returns default MSW data)
    await page.goto("/search");
    await expect(page.getByText(/搜索|Search/).first()).toBeVisible();

    const searchInput = getSearchPageInput(page);
    await expect(searchInput).toBeVisible();
    await searchInput.fill("OLED");
    // The debounced auto-search triggers after 300ms; wait + fetch + render
    await page.waitForTimeout(2000);

    // Step 2: Verify search results show a patent from default MSW data
    // pat_001 has title "Novel Blue Emitter Material for OLED Devices"
    await expect(
      page.getByText("Novel Blue Emitter Material for OLED Devices", { exact: false }).first(),
    ).toBeVisible({ timeout: 10000 });

    // Click the patent title to navigate to detail page
    await page.getByText("Novel Blue Emitter Material for OLED Devices").first().click();

    // Step 3: Verify patent detail page for pat_001 (handled by MSW)
    await expect(page).toHaveURL(/\/patents\/pat_001/);
    await expect(
      page.getByText("Novel Blue Emitter Material for OLED Devices").first(),
    ).toBeVisible({ timeout: 10000 });

    // Verify publication number
    await expect(page.getByText("CN115321456A").first()).toBeVisible();

    // Verify assignee
    await expect(
      page.getByText("Samsung SDI Co., Ltd.").first(),
    ).toBeVisible();

    // Verify claims section is present
    await expect(
      page.getByText(/权利要求|Claims/i).first(),
    ).toBeVisible();

    // Step 4: Navigate to infringement-watch for FTO assessment
    // (MSW handler returns default alerts data)
    await page.goto("/infringement-watch");

    // Verify infringement page loaded
    await expect(
      page.getByText(/侵权|Infringement/i).first(),
    ).toBeVisible({ timeout: 10000 });

    // Verify at least one alert is shown (MSW returns 7 alerts)
    await expect(
      page.getByText(/HIGH|高风险/i).first(),
    ).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  Scenario 2: 分子搜索与对比
//  Input SMILES → Similarity search → Molecule detail → Add property comparison
// ═══════════════════════════════════════════════════════════════════════════════

test.describe("Scenario 2: 分子搜索与对比", () => {
  test("SMILES 输入 → 分子详情 → 属性对比", async ({ page }) => {
    // ── Mock molecule endpoints using a single route handler ──
    // Must register BEFORE navigation so it catches all molecule calls.
    await page.route(/\/api\/v1\/molecules/, async (route) => {
      const url = route.request().url();
      if (url.includes("mol_001")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(SINGLE_OK(FIXTURE_MOLECULE_CBP)),
        });
      } else if (url.includes("mol_002")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(SINGLE_OK(FIXTURE_MOLECULE_NPB)),
        });
      } else {
        // Molecule list (GET /molecules?page=...)
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(
            MOLECULE_LIST_OK([FIXTURE_MOLECULE_CBP, FIXTURE_MOLECULE_NPB]),
          ),
        });
      }
    });

    // Mock patent search to return results for "OLED" query
    await mockJson(page, /\/api\/v1\/patents\/search/, {
      ...PATENT_SEARCH_OK([
        {
          id: "pat_001",
          publicationNumber: "CN115321456A",
          title: "Novel Blue Emitter Material for OLED Devices",
          assignee: "Samsung SDI Co., Ltd.",
          legalStatus: "pending",
          filingDate: "2022-03-15",
        },
      ]),
    });

    // Step 1: Navigate to search page and trigger search
    await page.goto("/search");
    const searchInput = getSearchPageInput(page);
    await expect(searchInput).toBeVisible();
    await searchInput.fill("OLED");
    // Wait for debounced auto-search (300ms debounce + fetch + render)
    await page.waitForTimeout(2000);

    // Step 2: Verify results show molecules (from mocked molecule list)
    await expect(
      page.getByText(/CBP|NPB/).first(),
    ).toBeVisible({ timeout: 10000 });

    // Step 3: Switch to Molecules tab
    const moleculesTabButton = page.getByRole("button", {
      name: /Molecules|分子/i,
    });
    if (await moleculesTabButton.count() > 0) {
      await moleculesTabButton.first().click();
      await page.waitForTimeout(500);
    }

    // Step 4: Click on CBP molecule name to navigate to detail
    const cbpLink = page.getByText("CBP").first();
    await expect(cbpLink).toBeVisible({ timeout: 10000 });
    await cbpLink.click();

    // Step 5: Verify molecule detail page
    await expect(page).toHaveURL(/\/molecules\/mol_001/);
    await expect(page.getByText("CBP").first()).toBeVisible({ timeout: 10000 });

    // Verify molecular weight is displayed
    await expect(
      page.getByText(/319\.41|Molecular Weight|分子量/i).first(),
    ).toBeVisible();

    // Verify SMILES is visible
    await expect(
      page.getByText(/c1ccc/).first(),
    ).toBeVisible();

    // Verify material properties table is visible
    await expect(
      page.getByText(/HOMO|LUMO|Triplet Energy/i).first(),
    ).toBeVisible();

    // Step 6: Add NPB molecule for property comparison
    // The PropertyComparison component has an input to add molecules
    const compareInput = page.getByPlaceholder(/SMILES|molecule ID/i).first();
    if (await compareInput.count() > 0) {
      await compareInput.fill("mol_002");

      // Click the Add button
      const addButton = page.getByRole("button", { name: /^Add$/i });
      if (await addButton.count() > 0) {
        await addButton.first().click();
        await page.waitForTimeout(1500);

        // Verify comparison section shows both molecules
        await expect(
          page.getByText(/Property Comparison|属性对比/i).first(),
        ).toBeVisible();
      }
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  Scenario 3: Dashboard 数据流
//  KPI display → Alert notifications → Export report
// ═══════════════════════════════════════════════════════════════════════════════

test.describe("Scenario 3: Dashboard 数据流", () => {
  test("KPI 展示 → 告警通知 → 导出报告", async ({ page }) => {
    // ── Mock dashboard metrics ──
    await mockJson(page, /\/api\/v1\/dashboard\/metrics/, {
      code: 0,
      message: "success",
      data: FIXTURE_DASHBOARD_METRICS,
    });

    // Step 1: Navigate to dashboard
    await page.goto("/dashboard");

    // Step 2: Verify dashboard title and KPI cards
    await expect(
      page.getByText(/管理驾驶舱|Executive Dashboard/i).first(),
    ).toBeVisible({ timeout: 15000 });

    // Verify KPI values are rendered
    await expect(page.getByText(/158/).first()).toBeVisible();

    // Verify KPI labels
    await expect(
      page.getByText(/专利总数|Total Patents/i).first(),
    ).toBeVisible();
    await expect(
      page.getByText(/有效专利|Active Patents/i).first(),
    ).toBeVisible();
    await expect(
      page.getByText(/高风险预警|High Risk/i).first(),
    ).toBeVisible();

    // Step 3: Verify chart components
    await expect(
      page.getByText(/月度申请趋势|Monthly Application Trend/i).first(),
    ).toBeVisible();
    await expect(
      page.getByText(/司法管辖区分布|Jurisdiction Breakdown/i).first(),
    ).toBeVisible();

    // Step 4: Verify upcoming deadlines section
    await expect(
      page.getByText(/即将到期|Upcoming Deadlines/i).first(),
    ).toBeVisible();

    // Step 5: Verify recent alerts section on dashboard
    await expect(
      page.getByText(/近期侵权预警|Recent Infringement/i).first(),
    ).toBeVisible();

    // Verify alert details are shown
    await expect(
      page.getByText(/HIGH|高风险/i).first(),
    ).toBeVisible();

    // Step 6: Click Export Report button
    const exportButton = page.getByRole("button", {
      name: /导出报告|Export Report/i,
    });
    if (await exportButton.count() > 0) {
      await exportButton.first().click();
      // Wait for export simulation (2s timeout in handler)
      await page.waitForTimeout(2500);
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  Scenario 4: 多语言切换
//  English → Chinese → Japanese → Korean → Verify nav label translations
// ═══════════════════════════════════════════════════════════════════════════════

test.describe("Scenario 4: 多语言切换", () => {
  test("英文 → 中文 → 日文 → 韩文 导航标签验证", async ({ page }) => {
    // Start in Chinese (default), then switch to English for first verification
    await page.goto("/dashboard");
    await expect(page.getByText("管理驾驶舱").first()).toBeVisible({ timeout: 15000 });

    // ═══ Switch to English via dropdown (same pattern as 80_theme_language.spec.ts) ═══
    const langSelect = page.locator('select[title="Switch Language"]');
    await expect(langSelect).toBeVisible();
    await langSelect.selectOption("en");
    await page.waitForTimeout(1000);

    // Verify dashboard title in English
    await expect(
      page.getByText("Executive Dashboard").first(),
    ).toBeVisible({ timeout: 15000 });

    // Sidebar navigation labels in English
    await expect(page.getByText("Dashboard").first()).toBeVisible();
    await expect(page.getByText("Search").first()).toBeVisible();
    await expect(page.getByText("Patent Mining").first()).toBeVisible();
    await expect(page.getByText("Infringement Watch").first()).toBeVisible();
    await expect(
      page.getByText("Portfolio Optimizer").first(),
    ).toBeVisible();

    // ═══ Chinese (zh-CN) ═══
    // Switch via language selector in TopBar
    await langSelect.selectOption("zh-CN");
    await page.waitForTimeout(1000);

    // Verify dashboard title in Chinese
    await expect(
      page.getByText("管理驾驶舱").first(),
    ).toBeVisible();

    // Sidebar navigation labels in Chinese
    await expect(page.getByText("搜索").first()).toBeVisible();
    await expect(page.getByText("专利挖掘").first()).toBeVisible();
    await expect(page.getByText("侵权监测").first()).toBeVisible();
    await expect(page.getByText("组合优化").first()).toBeVisible();

    // ═══ Japanese (ja) ═══
    await langSelect.selectOption("ja");
    await page.waitForTimeout(1000);

    // Sidebar navigation labels in Japanese
    await expect(page.getByText("検索").first()).toBeVisible();
    await expect(page.getByText("特許マイニング").first()).toBeVisible();
    await expect(page.getByText("侵害監視").first()).toBeVisible();

    // ═══ Korean (ko) ═══
    await langSelect.selectOption("ko");
    await page.waitForTimeout(1000);

    // Sidebar navigation labels in Korean
    await expect(page.getByText("검색").first()).toBeVisible();
    await expect(page.getByText("특허 마이닝").first()).toBeVisible();
    await expect(page.getByText("침해 모니터링").first()).toBeVisible();
  });
});

// ═══════════════════════════════════════════════════════════════════════════════
//  Scenario 5: 环境模式切换
//  Mock mode → Switch to Proxy mode → Verify environment banner
// ═══════════════════════════════════════════════════════════════════════════════

test.describe("Scenario 5: 环境模式切换", () => {
  test("Mock 模式验证 → Proxy 模式切换 → 环境提示条", async ({ page }) => {
    // Step 1: Start in mock mode (default)
    await page.goto("/dashboard");
    await expect(
      page.getByText(/管理驾驶舱|Executive Dashboard/i).first(),
    ).toBeVisible({ timeout: 15000 });

    // Step 2: Verify the environment banner shows Mock mode message
    // The EnvironmentBanner displays "Mock 模式 — 数据为模拟数据" in mock mode
    await expect(
      page.getByText("Mock 模式", { exact: false }).first(),
    ).toBeVisible({ timeout: 5000 });

    // Step 3: Click API Mode Switcher (Settings gear icon in TopBar)
    // The button has a `title` attribute like "API 模式: Mock (本地 Mock 数据)"
    const settingsButton = page.locator('button[title*="API"]').first();
    await expect(settingsButton).toBeVisible();
    await settingsButton.click();
    await page.waitForTimeout(500);

    // Step 4: Select Proxy mode from the dropdown menu
    const proxyOption = page.locator('button:has-text("Proxy")').first();
    await expect(proxyOption).toBeVisible();
    await proxyOption.click();

    // Clicking Proxy mode calls setApiMode('proxy') which triggers
    // window.location.reload(). Wait for the new page to load.
    await page.waitForLoadState("load", { timeout: 15000 });
    await page.waitForTimeout(2000);

    // Step 5: After reload, verify the environment banner shows Proxy message
    await expect(
      page.getByText("代理模式", { exact: false }).first(),
    ).toBeVisible({ timeout: 10000 });

    // Verify the text about localhost:8080 is visible
    await expect(
      page.getByText("localhost:8080", { exact: false }).first(),
    ).toBeVisible({ timeout: 5000 });

    // Step 6: Switch back to Mock mode for clean state
    // Re-open the settings menu
    const gearButton2 = page.locator('button[title*="API"]').first();
    if (await gearButton2.count() > 0) {
      await gearButton2.click();
      await page.waitForTimeout(500);
    }

    const mockOption = page.locator('button:has-text("Mock"):not(:has-text("Proxy"))').first();
    if (await mockOption.count() > 0) {
      await mockOption.click();
      await page.waitForLoadState("load", { timeout: 15000 });
      await page.waitForTimeout(2000);

      // Verify we're back in mock mode
      await expect(
        page.getByText("Mock 模式", { exact: false }).first(),
      ).toBeVisible({ timeout: 10000 });
    }
  });
});
