import { expect, test } from "@playwright/test";

test("notifications panel triage flow and events handoff", async ({ page }) => {
  const loginResponse = await page.request.post("/api/auth/login", { data: { token: "e2e-viewer-token" } });
  expect(loginResponse.status()).toBe(200);

  await page.goto("/");

  await page.getByRole("button", { name: "Notifications" }).click();
  await expect(page.getByRole("heading", { name: "Notifications" })).toBeVisible();

  await expect(page.getByRole("button", { name: "Mark all read" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Clear cache" })).toBeVisible();

  await page.getByPlaceholder("Filter by reason, message, or source").fill("no-match-e2e-filter");
  await expect(page.getByText("No notifications match your current filters.")).toBeVisible();

  await page.getByRole("button", { name: "Open events view" }).click();
  await expect(page.getByRole("heading", { name: "Events" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Notifications" })).toHaveCount(0);
});

test("profile panel authenticates with normalized bearer token and logs out", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Profile" }).click();
  await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();

  const tokenInput = page.getByPlaceholder("Paste API token");
  const authenticateButton = page.getByRole("button", { name: "Authenticate" });

  await tokenInput.fill("Bearer");
  await expect(authenticateButton).toBeDisabled();

  await tokenInput.fill("Bearer e2e-viewer-token");
  await authenticateButton.click();
  await expect(page.getByText("Session authenticated as viewer.")).toBeVisible();

  const sessionAfterLogin = await page.request.get("/api/auth/session", {
    headers: { Authorization: "Bearer e2e-viewer-token" },
  });
  expect(sessionAfterLogin.status()).toBe(200);
  const sessionPayload = await sessionAfterLogin.json();
  expect(sessionPayload).toBeDefined();
  expect(sessionPayload.user).toBeDefined();
  expect(sessionPayload.user.role).toBe("viewer");

  await page.getByRole("button", { name: "Logout" }).click();
  await expect(page.getByText("Session logged out.")).toBeVisible();

  const sessionAfterLogout = await page.request.get("/api/auth/session");
  expect(sessionAfterLogout.status()).toBe(200);
  const loggedOutPayload = await sessionAfterLogout.json();
  expect(loggedOutPayload.authenticated).toBe(false);
});
