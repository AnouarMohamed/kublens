import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

const operatorToken = "e2e-operator-token";

async function loginWithToken(request: APIRequestContext, token: string) {
  const response = await request.post("/api/auth/login", { data: { token } });
  expect(response.status()).toBe(200);
}

async function openView(page: Page, query: string, heading: string) {
  const search = page.getByPlaceholder("search views (/)");
  await search.fill(query);
  await page.getByRole("button", { name: "Execute search" }).click();
  await expect(page.getByRole("heading", { name: heading })).toBeVisible();
}

test("incident and remediation browser flows render operational state", async ({ page }) => {
  await loginWithToken(page.request, operatorToken);

  await page.goto("/");
  await openView(page, "incidents", "Incident Commander");

  await expect(page.getByText("Total incidents")).toBeVisible();
  await page.getByRole("button", { name: "Trigger Incident" }).click();

  await expect(page.getByText("Runbook completion")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Incident Replay" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Evidence Bundle" })).toBeVisible();

  const nextRunbookButton = page.getByRole("button", { name: /^(Start|Mark) / }).first();
  if (await nextRunbookButton.isVisible()) {
    await nextRunbookButton.click();
    await expect(page.getByText(/updated to/)).toBeVisible();
  }

  await openView(page, "remediation", "Safe Auto-Remediation");

  await expect(page.getByText("Operator guidance").or(page.getByText("No remediation proposals match"))).toBeVisible();
  await page.getByRole("button", { name: "Generate Proposals" }).click();

  await expect(page.getByText("GitOps Mode")).toBeVisible();

  await page.getByRole("button", { name: "Prepare GitOps" }).last().click();
  await expect(page.getByText("YAML Artifact").or(page.getByText("Instructions"))).toBeVisible();
});
