import { expect, test, type APIResponse } from "@playwright/test";

const viewerToken = "e2e-viewer-token";
const operatorToken = "e2e-operator-token";

type IncidentStep = {
  id: string;
  status: "pending" | "in_progress" | "done" | "skipped";
};

type Incident = {
  id: string;
  status: "open" | "resolved";
  runbook: IncidentStep[];
};

type Postmortem = {
  id: string;
  incidentId: string;
};

type IncidentReplay = {
  incidentId: string;
  frames: Array<{ offsetMinutes: number }>;
};

type IncidentEvidenceBundle = {
  incidentId: string;
  audit: Array<{ id: string }>;
  markdown: string;
};

type MemoryRunbook = {
  id: string;
  title: string;
};

type RemediationProposal = {
  id: string;
  kind: "restart_pod" | "cordon_node" | "rollback_deployment";
};

type RemediationGitOpsArtifact = {
  proposalId: string;
  artifact: {
    supportLevel: "patch_ready" | "advisory";
    artifactBody: string;
  };
};

type RightsizingOverview = {
  items: Array<{
    id: string;
    status: "overprovisioned" | "underprovisioned" | "missing_guardrails" | "balanced";
    artifact?: { supportLevel: "patch_ready" | "advisory" };
  }>;
};

type MemoryFix = {
  id: string;
  incidentId: string;
};

async function expectJSON<T>(response: APIResponse, status: number): Promise<T> {
  const text = await response.text();
  expect(response.status(), `Expected status ${status}, got ${response.status()} with body: ${text}`).toBe(status);
  return JSON.parse(text) as T;
}

test("incident lifecycle and memory flow remain operational", async ({ request }) => {
  const health = await request.get("/api/healthz");
  expect(health.status()).toBe(200);
  expect(health.headers()["content-security-policy"] ?? "").toContain("default-src");
  expect(health.headers()["x-frame-options"]).toBe("DENY");
  expect(health.headers()["x-content-type-options"]).toBe("nosniff");

  const created = await expectJSON<Incident>(
    await request.post("/api/incidents", {
      headers: { Authorization: `Bearer ${viewerToken}` },
      data: {},
    }),
    201,
  );
  expect(created.id).toBeTruthy();
  expect(created.runbook.length).toBeGreaterThan(0);

  let currentIncident = created;
  for (const step of currentIncident.runbook) {
    const encodedIncidentID = encodeURIComponent(currentIncident.id);
    const encodedStepID = encodeURIComponent(step.id);

    if (step.status === "pending") {
      currentIncident = await expectJSON<Incident>(
        await request.patch(`/api/incidents/${encodedIncidentID}/steps/${encodedStepID}`, {
          headers: {
            Authorization: `Bearer ${operatorToken}`,
            "Content-Type": "application/json",
          },
          data: { status: "in_progress" },
        }),
        200,
      );
    }

    if (step.status === "done" || step.status === "skipped") {
      continue;
    }

    currentIncident = await expectJSON<Incident>(
      await request.patch(`/api/incidents/${encodedIncidentID}/steps/${encodedStepID}`, {
        headers: {
          Authorization: `Bearer ${operatorToken}`,
          "Content-Type": "application/json",
        },
        data: { status: "done" },
      }),
      200,
    );
  }

  const resolved = await expectJSON<Incident>(
    await request.post(`/api/incidents/${encodeURIComponent(currentIncident.id)}/resolve`, {
      headers: { Authorization: `Bearer ${operatorToken}` },
      data: {},
    }),
    200,
  );
  expect(resolved.status).toBe("resolved");

  const postmortem = await expectJSON<Postmortem>(
    await request.post(`/api/incidents/${encodeURIComponent(currentIncident.id)}/postmortem`, {
      headers: { Authorization: `Bearer ${operatorToken}` },
      data: {},
    }),
    201,
  );
  expect(postmortem.incidentId).toBe(currentIncident.id);

  const replay = await expectJSON<IncidentReplay>(
    await request.get(`/api/incidents/${encodeURIComponent(currentIncident.id)}/replay`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(replay.incidentId).toBe(currentIncident.id);
  expect(replay.frames.length).toBeGreaterThan(0);

  const evidence = await expectJSON<IncidentEvidenceBundle>(
    await request.get(`/api/incidents/${encodeURIComponent(currentIncident.id)}/evidence`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(evidence.incidentId).toBe(currentIncident.id);
  expect(evidence.audit.length).toBeGreaterThan(0);
  expect(evidence.markdown).toContain("Incident Evidence Bundle");

  const proposals = await expectJSON<RemediationProposal[]>(
    await request.post("/api/remediation/propose", {
      headers: { Authorization: `Bearer ${viewerToken}` },
      data: {},
    }),
    200,
  );
  const restartProposal = proposals.find((item) => item.kind === "restart_pod");
  expect(restartProposal).toBeTruthy();

  const generatedArtifact = await expectJSON<RemediationGitOpsArtifact>(
    await request.post(`/api/remediation/${encodeURIComponent(restartProposal!.id)}/gitops`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
      data: {},
    }),
    200,
  );
  expect(generatedArtifact.proposalId).toBe(restartProposal!.id);
  expect(generatedArtifact.artifact.artifactBody.length).toBeGreaterThan(0);

  const fetchedArtifact = await expectJSON<RemediationGitOpsArtifact>(
    await request.get(`/api/remediation/${encodeURIComponent(restartProposal!.id)}/gitops`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(fetchedArtifact.proposalId).toBe(restartProposal!.id);

  const rightsizing = await expectJSON<RightsizingOverview>(
    await request.get("/api/rightsizing", {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(rightsizing.items.length).toBeGreaterThan(0);
  expect(
    rightsizing.items.some(
      (item) =>
        item.status !== "balanced" &&
        (item.artifact?.supportLevel === "patch_ready" || item.artifact?.supportLevel === "advisory"),
    ),
  ).toBeTruthy();

  const runbookName = `e2e-runbook-${Date.now()}`;
  const createdRunbook = await expectJSON<MemoryRunbook>(
    await request.post("/api/memory/runbooks", {
      headers: {
        Authorization: `Bearer ${operatorToken}`,
        "Content-Type": "application/json",
      },
      data: {
        title: runbookName,
        tags: ["e2e", "incident"],
        description: "Automated runbook for e2e lifecycle coverage.",
        steps: ["Inspect incident", "Apply fix", "Verify recovery"],
      },
    }),
    201,
  );
  expect(createdRunbook.title).toContain("e2e-runbook-");

  const runbookSearch = await expectJSON<MemoryRunbook[]>(
    await request.get(`/api/memory/runbooks?q=${encodeURIComponent(runbookName)}`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(runbookSearch.some((item) => item.id === createdRunbook.id)).toBeTruthy();

  const updated = await expectJSON<MemoryRunbook>(
    await request.put(`/api/memory/runbooks/${encodeURIComponent(createdRunbook.id)}`, {
      headers: {
        Authorization: `Bearer ${operatorToken}`,
        "Content-Type": "application/json",
      },
      data: {
        title: `${runbookName}-updated`,
        tags: ["e2e", "incident"],
        description: "Updated runbook for e2e lifecycle coverage.",
        steps: ["Inspect incident", "Apply fix", "Verify recovery", "Document postmortem"],
      },
    }),
    200,
  );
  expect(updated.title).toContain("-updated");

  const createdFix = await expectJSON<MemoryFix>(
    await request.post("/api/memory/fixes", {
      headers: {
        Authorization: `Bearer ${operatorToken}`,
        "Content-Type": "application/json",
      },
      data: {
        incidentId: currentIncident.id,
        proposalId: `e2e-proposal-${Date.now()}`,
        title: "E2E lifecycle fix record",
        description: "Recorded by Playwright e2e test.",
        resource: "default/synthetic-target",
        kind: "rollback_deployment",
      },
    }),
    201,
  );
  expect(createdFix.incidentId).toBe(currentIncident.id);

  const listedFixes = await expectJSON<MemoryFix[]>(
    await request.get("/api/memory/fixes", {
      headers: { Authorization: `Bearer ${viewerToken}` },
    }),
    200,
  );
  expect(listedFixes.some((item) => item.id === createdFix.id)).toBeTruthy();
});
