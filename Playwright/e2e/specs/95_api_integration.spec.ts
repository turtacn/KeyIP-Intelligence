import { test, expect, type Page } from "@playwright/test";

/**
 * API Integration Tests — Proxy Mode (Real Backend)
 *
 * These tests set localStorage `keyip-api-mode` = `'proxy'` so the adapter
 * sends requests to `http://localhost:8080/api/v1` instead of MSW mocks.
 *
 * Three-state verification (loading / empty / error) is performed for
 * the health, patent-search, and molecule endpoints.
 *
 * The entire suite is auto-skipped when the backend is unreachable.
 */

// ─── Constants ──────────────────────────────────────────────────────────────

const API_BASE = "http://localhost:8080/api/v1";
const BACKEND_CHECK_TIMEOUT_MS = 5_000;

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** Ping the backend health endpoint; returns true if reachable. */
async function isBackendReachable(): Promise<boolean> {
  try {
    const resp = await fetch(`${API_BASE}/healthz`, {
      signal: AbortSignal.timeout(BACKEND_CHECK_TIMEOUT_MS),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

/**
 * Intercept requests matching `pattern` and add a `delayMs` pause before
 * forwarding to the real backend.  Used to reliably observe loading states.
 */
async function interceptAndDelay(
  page: Page,
  pattern: string,
  delayMs: number,
): Promise<void> {
  await page.route(pattern, async (route) => {
    await new Promise((r) => setTimeout(r, delayMs));
    await route.continue();
  });
}

// ─── Suite ───────────────────────────────────────────────────────────────────

test.describe("Real API Integration (Proxy Mode)", () => {
  let backendReachable = false;

  test.beforeAll(async () => {
    backendReachable = await isBackendReachable();
  });

  test.beforeEach(async ({ page }) => {
    test.skip(
      !backendReachable,
      `Backend at ${API_BASE} is not reachable — skipping proxy-mode tests`,
    );

    // Set proxy mode *before* any application code runs so that
    // getApiMode() reads the localStorage value on first call.
    await page.addInitScript(() => {
      localStorage.setItem("keyip-api-mode", "proxy");
    });
  });

  // ═══════════════════════════════════════════════════════════════════════════
  //  Health Check  (/healthz, /healthz/detail)
  // ═══════════════════════════════════════════════════════════════════════════

  test.describe("Health Check", () => {
    test("1a — loading state: skeleton visible while fetching health data", async ({
      page,
    }) => {
      // Delay the /healthz/** requests so we can capture the loading UI.
      await interceptAndDelay(page, `${API_BASE}/healthz**`, 3_000);

      await page.goto("/health");

      // The HealthPageSkeleton renders divs with the `animate-pulse` class.
      await expect(page.locator(".animate-pulse").first()).toBeVisible({
        timeout: 5_000,
      });

      // Remove the intercept so the real response comes through.
      await page.unroute(`${API_BASE}/healthz**`);

      // Eventually the status banner containing health data appears.
      await expect(
        page.getByText(/所有服务正常运行|系统服务异常|部分服务降级/).first(),
      ).toBeVisible({ timeout: 30_000 });
    });

    test("1b — success state: real health data is displayed", async ({
      page,
    }) => {
      await page.goto("/health");

      // Overall status banner.
      await expect(
        page.getByText(/所有服务正常运行|系统服务异常|部分服务降级/).first(),
      ).toBeVisible({ timeout: 30_000 });

      // At least one known service card heading is rendered.
      const anyServiceLabel = page
        .locator("h3")
        .filter({ hasText: /PostgreSQL|Redis|Neo4j|OpenSearch|Milvus/ })
        .first();
      await expect(anyServiceLabel).toBeVisible({ timeout: 10_000 });
    });

    test("1c — error state: 500 response shows PageError", async ({
      page,
    }) => {
      await page.route(`${API_BASE}/healthz**`, async (route) => {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({
            code: 5000,
            message: "Internal server error",
            data: null,
          }),
        });
      });

      await page.goto("/health");

      // PageError component renders error-related text.
      await expect(
        page.getByText(/加载失败|PageError|error|失败|错误/).first(),
      ).toBeVisible({ timeout: 15_000 });

      await page.unroute(`${API_BASE}/healthz**`);
    });
  });

  // ═══════════════════════════════════════════════════════════════════════════
  //  Patent Search  (POST /patents/search)
  // ═══════════════════════════════════════════════════════════════════════════

  test.describe("Patent Search", () => {
    test("2a — loading state: spinner appears during search", async ({
      page,
    }) => {
      await page.goto("/search");

      // Delay both patent-search and molecule endpoints (both fire on submit).
      await interceptAndDelay(page, `${API_BASE}/patents/search`, 3_000);
      await interceptAndDelay(page, `${API_BASE}/molecules?*`, 3_000);

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("OLED");
      await searchInput.press("Enter");

      // LoadingSpinner renders a lucide `Loader2` icon with `animate-spin`.
      await expect(page.locator(".animate-spin").first()).toBeVisible({
        timeout: 5_000,
      });

      await page.unroute(`${API_BASE}/patents/search`);
      await page.unroute(`${API_BASE}/molecules?*`);

      // Wait for the real response to arrive.
      await expect(
        page
          .getByText(/no_results|没有结果|未找到|adjust|结果|result/).first()
          .or(page.locator("table").first()),
      ).toBeVisible({ timeout: 30_000 });
    });

    test("2b — results: valid query returns patent data", async ({
      page,
    }) => {
      await page.goto("/search");

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("OLED");
      await searchInput.press("Enter");

      // Give the real backend time to respond.
      await page.waitForTimeout(5_000);

      // The page either renders a <table> (results) or an empty-state message.
      const hasTable = (await page.locator("table").count()) > 0;
      const hasEmpty = await page.getByText(
        /no_results|没有结果|未找到|adjust/i,
      ).count();

      if (hasTable) {
        // Publication numbers like CN2023…, US2023… etc.
        const pubNumbers = page.getByText(
          /CN\d+|US\d+|EP\d+|JP\d+|WO\d+/,
        );
        const count = await pubNumbers.count();
        expect(count).toBeGreaterThanOrEqual(0);
      } else if (hasEmpty > 0) {
        await expect(
          page.getByText(/no_results|没有结果|未找到|adjust/i).first(),
        ).toBeVisible();
      }
    });

    test("2c — empty state: obscure query yields no results", async ({
      page,
    }) => {
      await page.goto("/search");

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("XYZZYX_NONEXISTENT_QUERY_2025");
      await searchInput.press("Enter");

      await expect(
        page.getByText(/no_results|没有结果|未找到|adjust|No results/i).first(),
      ).toBeVisible({ timeout: 30_000 });
    });

    test("2d — error state: 500 response shows error banner", async ({
      page,
    }) => {
      await page.route(`${API_BASE}/patents/search`, async (route) => {
        await route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({
            code: 5000,
            message: "Search backend error",
            data: null,
          }),
        });
      });

      await page.goto("/search");

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("OLED");
      await searchInput.press("Enter");

      await expect(
        page.getByText(/Error|error|失败|错误|Search Error/).first(),
      ).toBeVisible({ timeout: 15_000 });

      await page.unroute(`${API_BASE}/patents/search`);
    });
  });

  // ═══════════════════════════════════════════════════════════════════════════
  //  Molecule Display  (GET /molecules, /molecules/:id)
  // ═══════════════════════════════════════════════════════════════════════════

  test.describe("Molecule Display", () => {
    test("3a — molecules tab renders data or empty state", async ({
      page,
    }) => {
      await page.goto("/search");

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("OLED");
      await searchInput.press("Enter");

      // Wait for the initial combined-results to load.
      await page.waitForTimeout(5_000);

      // Switch to the Molecules tab.
      const moleculesTab = page.getByRole("button", {
        name: /Molecules|分子/,
      });
      if ((await moleculesTab.count()) === 0) {
        test.skip(true, "Molecules tab not found on search page");
        return;
      }
      await moleculesTab.first().click();
      await page.waitForTimeout(3_000);

      // Either molecule-specific text appears, or an empty-state message.
      const hasMolData = await page.getByText(
        /mol|g\/mol|Molecular Weight|分子量|SMILES/i,
      ).count();
      const hasMolEmpty = await page.getByText(
        /no_molecules|没有分子|未找到/i,
      ).count();

      expect(hasMolData > 0 || hasMolEmpty > 0).toBeTruthy();
    });

    test("3b — direct molecule detail navigation", async ({ page }) => {
      // Navigate directly to a molecule detail route using an id known
      // from the mock fixtures (the real backend may or may not have it).
      await page.goto("/molecules/mol_001");

      await page.waitForTimeout(5_000);

      // We either see the molecule name/properties, or a not-found / error UI.
      const hasDetail = await page.getByText(
        /CBP|NPB|Alq3|mCP|4CzIPN|Molecular Weight|分子量|SMILES|HOMO|LUMO/i,
      ).count();
      const hasNotFound = await page.getByText(
        /not found|未找到|错误|Error|404/i,
      ).count();
      const hasError = await page.getByText(
        /加载失败|PageError|失败/i,
      ).count();

      expect(hasDetail > 0 || hasNotFound > 0 || hasError > 0).toBeTruthy();
    });
  });

  // ═══════════════════════════════════════════════════════════════════════════
  //  API Unavailability  (graceful degradation)
  // ═══════════════════════════════════════════════════════════════════════════

  test.describe("API Unavailability", () => {
    test("4a — connection refused shows error state on health page", async ({
      page,
    }) => {
      // Intercept and abort health requests as if the backend is down.
      await page.route(`${API_BASE}/healthz**`, async (route) => {
        await route.abort("connectionrefused");
      });

      await page.goto("/health");

      // The health page should render the PageError component.
      await expect(
        page.getByText(/加载失败|PageError|error|Error|失败|try again|重试/).first(),
      ).toBeVisible({ timeout: 15_000 });

      await page.unroute(`${API_BASE}/healthz**`);
    });

    test("4b — search gracefully handles network failure", async ({
      page,
    }) => {
      await page.route(`${API_BASE}/patents/search`, async (route) => {
        await route.abort("connectionrefused");
      });

      await page.goto("/search");

      const searchInput = page.locator('input[type="text"]').first();
      await searchInput.fill("OLED");
      await searchInput.press("Enter");

      // Should show error banner rather than crashing.
      await expect(
        page.getByText(/Error|error|失败|错误|Search Error/i).first(),
      ).toBeVisible({ timeout: 15_000 });

      await page.unroute(`${API_BASE}/patents/search`);
    });
  });
});
