import { useCallback, useEffect, useMemo, useState } from "react";
import { useAsyncResource } from "../../app/hooks/useAsyncResource";
import { KpiStrip } from "../../components/KpiStrip";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import type { GitOpsArtifact, RightsizingOverview, RightsizingStatus } from "../../types";

export default function RightsizingView() {
  const { can } = useAuthSession();
  const canRead = can("read");

  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [showBalanced, setShowBalanced] = useState(false);
  const [copyMessage, setCopyMessage] = useState<string | null>(null);

  const loadRightsizingOverview = useCallback((signal: AbortSignal) => api.getRightsizingOverview(signal), []);

  const {
    data: overview,
    isLoading,
    error,
    load,
  } = useAsyncResource<RightsizingOverview | null>({
    loader: loadRightsizingOverview,
    fallbackError: "Failed to load rightsizing overview",
    initialData: null,
    enabled: canRead,
    disabledData: null,
    disabledError: "Authenticate to view rightsizing recommendations.",
  });

  useEffect(() => {
    if (!overview) {
      setSelectedID(null);
      return;
    }
    setSelectedID((current) =>
      current && overview.items.some((item) => item.id === current) ? current : (overview.items[0]?.id ?? null),
    );
  }, [overview]);

  useEffect(() => {
    if (!copyMessage) {
      return;
    }
    const timeout = window.setTimeout(() => setCopyMessage(null), 1800);
    return () => window.clearTimeout(timeout);
  }, [copyMessage]);

  const visibleItems = useMemo(() => {
    if (!overview) {
      return [];
    }
    return showBalanced ? overview.items : overview.items.filter((item) => item.status !== "balanced");
  }, [overview, showBalanced]);

  const selected = useMemo(
    () => visibleItems.find((item) => item.id === selectedID) ?? visibleItems[0] ?? null,
    [selectedID, visibleItems],
  );

  const kpiItems = useMemo(() => {
    if (!overview) {
      return [
        { label: "Savings", value: "0" },
        { label: "Under", value: "0" },
        { label: "Missing", value: "0" },
        { label: "Reclaim", value: "0m / 0Mi" },
      ];
    }

    return [
      { label: "Savings", value: String(overview.savingsOpportunities), tone: "healthy" as const },
      { label: "Under", value: String(overview.underprovisioned), tone: "critical" as const },
      { label: "Missing", value: String(overview.missingGuardrails), tone: "warning" as const },
      {
        label: "Reclaim",
        value: `${overview.reclaimableCpu} / ${overview.reclaimableMemory}`,
        tone: "warning" as const,
      },
    ];
  }, [overview]);

  const copyArtifact = useCallback(async (artifact: GitOpsArtifact) => {
    if (!artifact.artifactBody) {
      return;
    }
    try {
      await navigator.clipboard.writeText(artifact.artifactBody);
      setCopyMessage(artifact.format === "yaml" ? "Patch copied." : "Advisory copied.");
    } catch {
      setCopyMessage("Copy failed.");
    }
  }, []);

  return (
    <div className="space-y-4">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight text-zinc-100">Rightsizing Advisor</h2>
          <p className="mt-1 text-sm text-zinc-400">
            Cost and capacity guidance from live usage, current requests and limits, and GitOps-ready change previews.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button onClick={() => setShowBalanced((value) => !value)} className="btn">
            {showBalanced ? "Hide balanced" : "Show balanced"}
          </button>
          <button onClick={() => void load()} disabled={isLoading} className="btn-primary">
            {isLoading ? "Refreshing" : "Refresh"}
          </button>
        </div>
      </header>

      {error && <div className="surface px-3 py-2 text-sm text-zinc-200">{error}</div>}
      {copyMessage && <div className="surface px-3 py-2 text-sm text-zinc-200">{copyMessage}</div>}

      <KpiStrip items={kpiItems} />

      {overview && (
        <>
          <section className="surface p-4">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">Efficiency Summary</p>
                <p className="mt-2 text-lg text-zinc-100">{overview.summary}</p>
                <p className="mt-1 text-xs text-zinc-500">Generated {formatTimestamp(overview.generatedAt)}</p>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <MetricTile label="Tracked" value={String(overview.items.length)} />
                <MetricTile label="Balanced" value={String(overview.balanced)} />
              </div>
            </div>
          </section>

          <section className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
            <div className="grid gap-4 md:grid-cols-2">
              {visibleItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedID(item.id)}
                  className={`surface p-4 text-left transition ${selected?.id === item.id ? "border-[var(--accent)]/60" : ""}`}
                >
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div>
                      <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">
                        {item.workloadKind ? `${item.workloadKind} ${item.workloadName}` : item.pod}
                      </p>
                      <h3 className="mt-2 text-lg font-semibold text-zinc-100">
                        {item.namespace}/{item.pod}
                      </h3>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <span className={`border px-2 py-1 text-[11px] uppercase ${statusBadgeClass(item.status)}`}>
                        {item.status.replaceAll("_", " ")}
                      </span>
                      <span className={`border px-2 py-1 text-[11px] uppercase ${riskBadgeClass(item.riskLevel)}`}>
                        {item.riskLevel}
                      </span>
                    </div>
                  </div>

                  <p className="mt-3 text-sm leading-relaxed text-zinc-400">{item.summary}</p>

                  <div className="mt-4 grid gap-3 sm:grid-cols-2">
                    <MetricTile label="Usage" value={`${item.cpuUsage} / ${item.memoryUsage}`} />
                    <MetricTile label="Reclaim" value={`${item.reclaimableCpu} / ${item.reclaimableMemory}`} />
                    <MetricTile label="Current" value={`${item.requestCpu} / ${item.requestMemory}`} />
                    <MetricTile
                      label="Recommended"
                      value={`${item.recommendedRequestCpu} / ${item.recommendedRequestMemory}`}
                    />
                  </div>
                </button>
              ))}

              {!isLoading && visibleItems.length === 0 && (
                <article className="surface p-4 md:col-span-2">
                  <p className="text-sm text-zinc-500">No recommendations match the current filter.</p>
                </article>
              )}
            </div>

            {selected && (
              <article className="surface p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">Selected Recommendation</p>
                    <h3 className="mt-2 text-xl font-semibold text-zinc-100">
                      {selected.namespace}/{selected.pod}
                    </h3>
                    <p className="mt-1 text-sm text-zinc-400">
                      QoS {selected.qosClass || "Unknown"} · {selected.containerCount} container
                      {selected.containerCount === 1 ? "" : "s"} · confidence {selected.confidence}%
                    </p>
                  </div>
                  <span className={`border px-2 py-1 text-[11px] uppercase ${statusBadgeClass(selected.status)}`}>
                    {selected.status.replaceAll("_", " ")}
                  </span>
                </div>

                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <MetricTile label="Current Requests" value={`${selected.requestCpu} / ${selected.requestMemory}`} />
                  <MetricTile label="Current Limits" value={`${selected.limitCpu} / ${selected.limitMemory}`} />
                  <MetricTile
                    label="Recommended Requests"
                    value={`${selected.recommendedRequestCpu} / ${selected.recommendedRequestMemory}`}
                  />
                  <MetricTile
                    label="Recommended Limits"
                    value={`${selected.recommendedLimitCpu} / ${selected.recommendedLimitMemory}`}
                  />
                </div>

                {selected.artifact ? (
                  <div className="mt-5 border-t border-zinc-700 pt-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">GitOps Preview</p>
                        <p className="mt-2 text-sm text-zinc-300">{selected.artifact.summary}</p>
                      </div>
                      <button onClick={() => void copyArtifact(selected.artifact!)} className="btn">
                        Copy {selected.artifact.format === "yaml" ? "Patch" : "Advisory"}
                      </button>
                    </div>

                    <div className="mt-4 grid gap-3 md:grid-cols-2">
                      <MetricTile label="Support" value={selected.artifact.supportLevel.replaceAll("_", " ")} />
                      <MetricTile label="Strategy" value={selected.artifact.strategy.replaceAll("_", " ")} />
                      <MetricTile label="Branch" value={selected.artifact.branchName} />
                      <MetricTile label="Target Path" value={selected.artifact.targetPath} />
                    </div>

                    <div className="mt-4 grid gap-3">
                      <MetricTile label="PR Title" value={selected.artifact.prTitle} />
                      <MetricTile label="Commit Message" value={selected.artifact.commitMessage} />
                    </div>

                    <div className="mt-4 border border-zinc-700 bg-zinc-950 p-3">
                      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">
                        {selected.artifact.format.toUpperCase()} Artifact
                      </p>
                      <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-xs leading-6 text-zinc-300">
                        {selected.artifact.artifactBody}
                      </pre>
                    </div>

                    <div className="mt-4">
                      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">Instructions</p>
                      <div className="mt-3 space-y-2">
                        {selected.artifact.instructions.map((instruction) => (
                          <div
                            key={instruction}
                            className="border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-300"
                          >
                            {instruction}
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="mt-5 border-t border-zinc-700 pt-4">
                    <p className="text-sm text-zinc-500">No GitOps artifact is needed for balanced recommendations.</p>
                  </div>
                )}
              </article>
            )}
          </section>
        </>
      )}
    </div>
  );
}

function MetricTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-zinc-700 bg-zinc-950 px-3 py-2">
      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{label}</p>
      <p className="mt-1 text-sm font-semibold text-zinc-100">{value}</p>
    </div>
  );
}

function statusBadgeClass(status: RightsizingStatus): string {
  if (status === "overprovisioned") {
    return "border-[var(--accent)] bg-[var(--accent-dim)] text-zinc-100";
  }
  if (status === "underprovisioned") {
    return "border-[var(--red)]/45 bg-[var(--red)]/10 text-[var(--red)]";
  }
  if (status === "missing_guardrails") {
    return "border-[var(--amber)]/50 bg-[var(--amber)]/10 text-[var(--amber)]";
  }
  return "border-zinc-700 bg-zinc-900 text-zinc-300";
}

function riskBadgeClass(risk: string): string {
  if (risk === "high") {
    return "border-[var(--red)]/45 bg-[var(--red)]/10 text-[var(--red)]";
  }
  if (risk === "medium") {
    return "border-[var(--amber)]/50 bg-[var(--amber)]/10 text-[var(--amber)]";
  }
  return "border-[var(--accent)] bg-[var(--accent-dim)] text-zinc-100";
}

function formatTimestamp(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}
