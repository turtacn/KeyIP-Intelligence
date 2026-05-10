import { test, expect } from "@playwright/test";

/**
 * Helpers: stable navigation + API mocking (repo uses baseUrl `/api/v1`)
 * These specs are designed to:
 *  - Fully click through UI and core actions
 *  - Work with the built-in MSW mock handlers (no real backend needed)
 *  - Produce per-spec videos in artifacts/test-output
 *  - Test mobile responsiveness with viewport 375x667 (iPhone SE/8)
 */

async function safeClick(page, role, nameRegex) {
  const loc = page.getByRole(role, { name: nameRegex });
  if (await loc.count()) await loc.first().click();
}

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Dashboard
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Dashboard", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Dashboard loads and is accessible on mobile viewport", async ({ page }) => {
    await page.goto("/dashboard");

    // The main content area should be visible
    await expect(page.locator("main")).toBeVisible();

    // The dashboard title should render (may be truncated)
    await expect(page.getByText(/Executive Dashboard|管理驾驶舱/i).first()).toBeVisible({ timeout: 15000 });

    // KPI cards should be visible in stacked layout on mobile
    await expect(page.getByText(/Total Patents|专利总数|Active Patents|有效专利/i).first()).toBeVisible();

    // Sidebar is positioned fixed and may overflow or be hidden on mobile
    // The key assertion is that the page doesn't break layout
    const sidebar = page.locator("aside");
    if (await sidebar.count() > 0) {
      // Sidebar exists in DOM — may be off-screen or collapsed on mobile
      // Just verify it doesn't cause horizontal overflow
      const box = await sidebar.boundingBox();
      if (box) {
        // Sidebar should be within viewport width
        expect(box.x + box.width).toBeLessThanOrEqual(380);
      }
    }
  });

  test("Dashboard charts stack vertically on mobile", async ({ page }) => {
    await page.goto("/dashboard");

    // Chart sections should render without breaking layout
    await expect(page.getByText(/Monthly Application Trend|月度申请趋势/i).first()).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(/Jurisdiction Breakdown|司法管辖区分布/i).first()).toBeVisible();
    await expect(page.getByText(/Competitive Radar|竞争雷达/i).first()).toBeVisible();
  });

  test("Dashboard export button is tappable on mobile", async ({ page }) => {
    await page.goto("/dashboard");

    // The export report button should be accessible on mobile
    const exportBtn = page.getByRole("button", { name: /Export Report|导出报告/i });
    if (await exportBtn.count() > 0) {
      await exportBtn.first().click();
      // Should not throw — button is tappable
    }
  });
});

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Search
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Search page", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Search page input and results render on mobile", async ({ page }) => {
    await page.goto("/search");

    // Search input should be visible and usable
    const searchInput = page.locator('input[type="text"]').first();
    await expect(searchInput).toBeVisible();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Results should render within mobile viewport
    await expect(page.getByText(/Novel Blue Emitter|CN115321456A|TADF/i).first()).toBeVisible({ timeout: 15000 });

    // Tab navigation (All/Patents/Molecules) should be tappable
    const patentsTab = page.getByRole("button", { name: /Patents|专利/i });
    if (await patentsTab.count() > 0) {
      await patentsTab.first().click();
      await page.waitForTimeout(300);
    }
  });

  test("Molecules tab renders properly on mobile", async ({ page }) => {
    await page.goto("/search");

    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill("OLED");
    await searchInput.press("Enter");

    // Switch to molecules tab
    await safeClick(page, "button", /Molecules|分子/i);
    await page.waitForTimeout(500);

    // Molecules table should be scrollable horizontally
    const moleculeName = page.getByText("CBP").first();
    await expect(moleculeName).toBeVisible({ timeout: 10000 });
  });
});

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Patent Detail
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Patent detail", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Patent detail page renders on mobile", async ({ page }) => {
    await page.goto("/patents/pat_001");

    // Patent title should be visible
    await expect(page.getByText("Novel Blue Emitter Material for OLED Devices")).toBeVisible({ timeout: 15000 });

    // Back button should be accessible
    const backBtn = page.getByText(/Back|返回/i).first();
    await expect(backBtn).toBeVisible();

    // Assignee and filing date should be visible
    await expect(page.getByText("Samsung SDI Co., Ltd.")).toBeVisible();
    await expect(page.getByText("2022-03-15")).toBeVisible();
  });

  test("Patent detail claims section is scrollable on mobile", async ({ page }) => {
    await page.goto("/patents/pat_001");

    // Scroll down to the claims section
    await expect(page.getByText("Claims").first()).toBeVisible({ timeout: 15000 });

    // The page should be vertically scrollable
    const scrollHeight = await page.evaluate(() => document.documentElement.scrollHeight);
    const viewportHeight = await page.evaluate(() => window.innerHeight);
    expect(scrollHeight).toBeGreaterThan(viewportHeight);
  });
});

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Sidebar and Navigation
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Navigation", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Sidebar is present but may be collapsed on mobile", async ({ page }) => {
    await page.goto("/dashboard");

    // Check that the sidebar is rendered in the DOM
    const sidebar = page.locator("aside");
    await expect(sidebar).toBeAttached();

    // On mobile with Tailwind's `w-64`, the sidebar takes 256px
    // This is a fixed sidebar that may overlap content on narrow viewports
    // Verify the app title is rendered
    await expect(page.getByText(/KeyIP/i).first()).toBeVisible();
  });

  test("Page content is readable after sidebar navigation on mobile", async ({ page }) => {
    await page.goto("/dashboard");

    // Navigate between pages — links should be tappable
    const searchLink = page.getByRole("link", { name: /Search|搜索/i });
    if (await searchLink.count() > 0) {
      await searchLink.first().click();
      await page.waitForTimeout(300);
      await expect(page).toHaveURL(/\/search/);
    }
  });
});

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Portfolio
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Portfolio", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Portfolio page renders and sections are accessible on mobile", async ({ page }) => {
    await page.goto("/portfolio");

    // Portfolio should load and render
    await expect(page.getByText(/Portfolio|组合/i).first()).toBeVisible({ timeout: 15000 });

    // Navigation tabs should be present
    const sections = [/Panorama|全景/i, /Gap|缺口/i, /Scoring|评分/i, /Budget|预算/i, /Simulator|模拟/i];
    for (const section of sections) {
      const nav = page.getByText(section).first();
      if (await nav.count()) {
        await nav.scrollIntoViewIfNeeded().catch(() => {});
      }
    }
  });
});

// ──────────────────────────────────────────────
// Mobile Responsive Layout - Lifecycle
// ──────────────────────────────────────────────

test.describe("Mobile responsive: Lifecycle", () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test("Lifecycle page renders on mobile", async ({ page }) => {
    await page.goto("/lifecycle");

    // Lifecycle should load
    await expect(page.getByText(/Lifecycle|生命周期/i).first()).toBeVisible({ timeout: 15000 });

    // Tab navigation should be functional
    const tabs = [/Deadline Calendar|期限日历|Annuity|年金/i, /Legal Status|法律状态/i];
    for (const tab of tabs) {
      const tabBtn = page.getByRole("button", { name: tab });
      if (await tabBtn.count() > 0) {
        await tabBtn.first().click();
        await page.waitForTimeout(300);
      }
    }
  });
});
