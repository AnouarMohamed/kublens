import { expect, test, type APIRequestContext, type APIResponse } from "@playwright/test";

const viewerToken = "e2e-viewer-token";
const operatorToken = "e2e-operator-token";
const adminToken = "e2e-admin-token";

async function loginWithToken(request: APIRequestContext, token: string): Promise<APIResponse> {
  const response = await request.post("/api/auth/login", { data: { token } });
  expect(response.status()).toBe(200);
  return response;
}

async function logoutSession(request: APIRequestContext) {
  const response = await request.post("/api/auth/logout", { data: {} });
  expect(response.status()).toBe(200);
}

function uniquePodName(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 10_000)}`;
}

async function expectStatus(response: Awaited<ReturnType<APIRequestContext["post"]>>, status: number) {
  const body = await response.text();
  expect(response.status(), `Expected ${status}, got ${response.status()} with body: ${body}`).toBe(status);
}

test("auth role matrix and policy gates", async ({ page }) => {
  const request = page.request;
  const operatorPodName = uniquePodName("e2e-operator-write");

  await loginWithToken(request, viewerToken);
  let session = await request.get("/api/auth/session", {
    headers: { Authorization: `Bearer ${viewerToken}` },
  });
  expect(session.status()).toBe(200);
  let sessionPayload = await session.json();
  expect(sessionPayload.user).toBeDefined();
  expect(sessionPayload.user.role).toBe("viewer");

  const viewerWrite = await request.post("/api/pods", {
    headers: { Authorization: `Bearer ${viewerToken}` },
    data: { namespace: "default", name: "e2e-viewer-attempt", image: "nginx:latest" },
  });
  expect(viewerWrite.status()).toBe(403);
  await logoutSession(request);

  await loginWithToken(request, operatorToken);
  session = await request.get("/api/auth/session", {
    headers: { Authorization: `Bearer ${operatorToken}` },
  });
  sessionPayload = await session.json();
  expect(sessionPayload.user).toBeDefined();
  expect(sessionPayload.user.role).toBe("operator");

  const operatorWrite = await request.post("/api/pods", {
    headers: { Authorization: `Bearer ${operatorToken}` },
    data: { namespace: "default", name: operatorPodName, image: "nginx:latest" },
  });
  await expectStatus(operatorWrite, 200);
  const operatorWritePayload = await operatorWrite.json();
  expect(operatorWritePayload.success).toBe(true);

  await request.delete(`/api/pods/default/${operatorPodName}`, {
    headers: { Authorization: `Bearer ${operatorToken}` },
  });
  await logoutSession(request);

  const adminLogin = await loginWithToken(request, adminToken);
  const adminSessionCookie = adminLogin
    .headersArray()
    .find((header) => header.name.toLowerCase() === "set-cookie")
    ?.value
    ?.split(";")
    .at(0);
  if (!adminSessionCookie) {
    throw new Error("expected auth session cookie in login response");
  }
  session = await request.get("/api/auth/session", {
    headers: { Authorization: `Bearer ${adminToken}` },
  });
  sessionPayload = await session.json();
  expect(sessionPayload.user).toBeDefined();
  expect(sessionPayload.user.role).toBe("admin");

  const csrfBlocked = await request.post("/api/pods", {
    headers: { Origin: "https://evil.example", Cookie: adminSessionCookie },
    data: { namespace: "default", name: "csrf-block", image: "nginx:latest" },
  });
  expect(csrfBlocked.status()).toBe(403);
  await logoutSession(request);
});
