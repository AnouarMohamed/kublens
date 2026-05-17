import { expect, test } from "@playwright/test";

test("dashboard loads and navigates between core views", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Cluster Overview" })).toBeVisible();
  await expect(page.getByRole("button", { name: /refresh/i })).toBeVisible();

  const search = page.getByPlaceholder("search views (/)");
  await search.fill("pods");
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.getByRole("heading", { name: "Pods" })).toBeVisible();
  await expect(page.getByText("kubectl get pods -A")).toBeVisible();

  await search.fill("diagnostics");
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.getByRole("heading", { name: "Diagnostics" })).toBeVisible();
  await expect(page.getByText("kubectl describe nodes")).toBeVisible();

  await search.fill("predictions");
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.getByRole("heading", { name: "Predictions" })).toBeVisible();
  await expect(page.getByText("kubectl get events -A --sort-by=.metadata.creationTimestamp")).toBeVisible();

  await search.fill("slo");
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.locator("main").getByRole("heading", { name: "SLO Center" })).toBeVisible();
  await expect(page.getByText(/Error-budget posture/i)).toBeVisible();

  await search.fill("rightsizing");
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.getByRole("heading", { name: "Rightsizing Advisor" })).toBeVisible();
  await expect(page.getByText(/Cost and capacity guidance/i)).toBeVisible();
});
