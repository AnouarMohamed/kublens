import { expect, test } from "@playwright/test";

test("overview loads within practical local budget", async ({ page }) => {
  const response = await page.goto("/", { waitUntil: "domcontentloaded" });
  expect(response?.ok()).toBeTruthy();

  await expect(
    page.getByRole("heading", { name: "Cluster Overview" })
  ).toBeVisible({ timeout: 10000 });

  const timing = await page.evaluate(() => {
    const nav = performance.getEntriesByType("navigation")[0] as
      | PerformanceNavigationTiming
      | undefined;
    if (!nav) {
      return null;
    }
    return {
      domContentLoadedMs: nav.domContentLoadedEventEnd - nav.startTime,
      loadEventMs: nav.loadEventEnd - nav.startTime,
    };
  });

  expect(timing).not.toBeNull();
  expect((timing?.domContentLoadedMs ?? 0) < 20000).toBe(true);
  expect((timing?.loadEventMs ?? 0) < 30000).toBe(true);
});
