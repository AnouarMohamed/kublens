import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { navigateToView } from "../../app/viewNavigation";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import type { AuditEntry, DiagnosticsResult, Incident, PredictionsResult, RemediationProposal } from "../../types";

interface ShiftSnapshot {
  diagnostics: DiagnosticsResult;
  predictions: PredictionsResult;
  incidents: Incident[];
  remediations: RemediationProposal[];
  audit: AuditEntry[];
}

export default function ShiftBrief() {
  const { can } = useAuthSession();
  const canRead = can("read");
  const [snapshot, setSnapshot] = useState<ShiftSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const copyResetRef = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!canRead) {
      setSnapshot(null);
      setError("Authenticate to view shift briefing data.");
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    try {
      const [diagnostics, predictions, incidents, remediations, audit] = await Promise.all([
        api.getDiagnostics(),
        api.getPredictions(),
        api.listIncidents(),
        api.listRemediation(),
        api.getAuditLog(25),
      ]);

      setSnapshot({
        diagnostics,
        predictions,
        incidents,
        remediations,
        audit: audit.items,
      });
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load shift brief");
    } finally {
      setIsLoading(false);
    }
  }, [canRead]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    return () => {
      if (copyResetRef.current !== null) {
        window.clearTimeout(copyResetRef.current);
      }
    };
  }, []);

  const criticalPredictions = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    return snapshot.predictions.items.filter((item) => item.riskScore >= 80).slice(0, 5);
  }, [snapshot]);

  const openIncidents = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    return snapshot.incidents.filter((incident) => incident.status === "open");
  }, [snapshot]);

  const pendingRemediations = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    return snapshot.remediations.filter((item) => item.status === "proposed" || item.status === "approved");
  }, [snapshot]);

  const recentMutations = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    return snapshot.audit
      .filter((entry) => ["POST", "PUT", "PATCH", "DELETE"].includes(entry.method.toUpperCase()))
      .slice(0, 8);
  }, [snapshot]);

  const markdown = useMemo(
    () =>
      snapshot
        ? buildShiftBriefMarkdown({
            snapshot,
            openIncidents,
            pendingRemediations,
            criticalPredictions,
            recentMutations,
          })
        : "",
    [criticalPredictions, openIncidents, pendingRemediations, recentMutations, snapshot],
  );

  const scheduleCopiedReset = useCallback(() => {
    if (copyResetRef.current !== null) {
      window.clearTimeout(copyResetRef.current);
    }
    copyResetRef.current = window.setTimeout(() => {
      setCopied(false);
      copyResetRef.current = null;
    }, 2000);
  }, []);

  const handleCopy = useCallback(async () => {
    if (!markdown) {
      return;
    }
    try {
      await navigator.clipboard.writeText(markdown);
      setCopied(true);
      scheduleCopiedReset();
    } catch {
      // Clipboard access is best effort.
    }
  }, [markdown, scheduleCopiedReset]);

  const handleDownload = useCallback(() => {
    if (!markdown) {
      return;
    }
    const blob = new Blob([markdown], { type: "text/markdown" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `shift-brief-${new Date().toISOString().slice(0, 10)}.md`;
    link.style.display = "none";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }, [markdown]);

  const handleOpenInAssistant = useCallback(() => {
    if (!markdown) {
      return;
    }
    navigateToView("assistant", {
      prefillMessage: `Summarize this shift brief and tell me what needs attention:\n\n${markdown}`,
    });
  }, [markdown]);

  return (
    <div className="space-y-5">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">Shift Brief</h2>
          <p className="text-sm text-zinc-400 mt-1">
            Live handoff view for on-call transitions: risk posture, active incidents, and recent changes.
          </p>
        </div>
        <div className="flex gap-2">
          <button onClick={() => void handleCopy()} className="btn-sm" disabled={!snapshot || isLoading || !canRead}>
            {copied ? "Copied" : "Copy"}
          </button>
          <button onClick={handleDownload} className="btn-sm" disabled={!snapshot || isLoading || !canRead}>
            Download .md
          </button>
          <button onClick={handleOpenInAssistant} className="btn-sm" disabled={!snapshot || isLoading || !canRead}>
            Open in Assistant
          </button>
          <button onClick={() => void load()} className="btn-sm" disabled={isLoading || !canRead}>
            {isLoading ? "Refreshing..." : "Refresh"}
          </button>
        </div>
      </header>

      {error && (
        <div className="rounded-md border border-red-500/35 bg-red-500/10 px-3 py-2 text-sm text-red-100">{error}</div>
      )}

      <section className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
        <BriefTile
          label="Health score"
          value={snapshot ? String(snapshot.diagnostics.healthScore) : "-"}
          note="Diagnostics engine"
        />
        <BriefTile label="Open incidents" value={String(openIncidents.length)} note="Incident commander queue" />
        <BriefTile label="Critical predictions" value={String(criticalPredictions.length)} note="Risk score >= 80" />
        <BriefTile
          label="Pending remediation"
          value={String(pendingRemediations.length)}
          note="Proposed or approved actions"
        />
      </section>

      <section className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <div className="rounded-md border border-zinc-800 bg-zinc-900/60 px-4 py-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">Top predicted risks</p>
          <div className="mt-2 space-y-2">
            {criticalPredictions.map((item) => (
              <div key={item.id} className="rounded-md border border-zinc-800 bg-zinc-900/70 px-3 py-2">
                <p className="text-sm font-medium text-zinc-100">
                  {item.namespace ? `${item.namespace}/` : ""}
                  {item.resource}
                </p>
                <p className="text-xs text-zinc-400 mt-1">{item.summary}</p>
                <p className="text-[11px] text-zinc-500 mt-1">
                  Risk {item.riskScore} | Confidence {item.confidence}
                </p>
              </div>
            ))}
            {criticalPredictions.length === 0 && (
              <p className="text-sm text-zinc-500">No critical predictions in the latest snapshot.</p>
            )}
          </div>
        </div>

        <div className="rounded-md border border-zinc-800 bg-zinc-900/60 px-4 py-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">Open incidents</p>
          <div className="mt-2 space-y-2">
            {openIncidents.slice(0, 6).map((incident) => (
              <div key={incident.id} className="rounded-md border border-zinc-800 bg-zinc-900/70 px-3 py-2">
                <p className="text-sm font-medium text-zinc-100">{incident.title}</p>
                <p className="text-xs text-zinc-400 mt-1">{incident.summary}</p>
                <p className="text-[11px] text-zinc-500 mt-1">
                  {incident.severity.toUpperCase()} | opened {formatTimestamp(incident.openedAt)}
                </p>
              </div>
            ))}
            {openIncidents.length === 0 && <p className="text-sm text-zinc-500">No open incidents.</p>}
          </div>
        </div>
      </section>

      <section className="rounded-md border border-zinc-800 bg-zinc-900/60 px-4 py-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">Recent mutating actions</p>
        <div className="mt-2 overflow-auto rounded-md border border-zinc-800">
          <table className="min-w-full text-left text-sm">
            <thead className="bg-zinc-900/60 text-xs uppercase tracking-wide text-zinc-400">
              <tr>
                <th className="px-3 py-2 font-semibold">Time</th>
                <th className="px-3 py-2 font-semibold">User</th>
                <th className="px-3 py-2 font-semibold">Action</th>
                <th className="px-3 py-2 font-semibold">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800 text-zinc-200">
              {recentMutations.map((entry) => (
                <tr key={`${entry.id}-${entry.timestamp}`}>
                  <td className="px-3 py-2 text-zinc-400">{formatTimestamp(entry.timestamp)}</td>
                  <td className="px-3 py-2">{entry.user || "unknown"}</td>
                  <td className="px-3 py-2 font-medium">{entry.action || `${entry.method} ${entry.path}`}</td>
                  <td className="px-3 py-2">{entry.status}</td>
                </tr>
              ))}
              {recentMutations.length === 0 && (
                <tr>
                  <td className="px-3 py-8 text-center text-zinc-500" colSpan={4}>
                    No recent mutating actions.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function BriefTile({ label, value, note }: { label: string; value: string; note: string }) {
  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900/60 px-3 py-2">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="mt-1 text-lg font-semibold text-zinc-100">{value}</p>
      <p className="text-xs text-zinc-400">{note}</p>
    </div>
  );
}

function formatTimestamp(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return "-";
  }
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  return date.toLocaleString();
}

function buildShiftBriefMarkdown({
  snapshot,
  openIncidents,
  pendingRemediations,
  criticalPredictions,
  recentMutations,
}: {
  snapshot: ShiftSnapshot;
  openIncidents: Incident[];
  pendingRemediations: RemediationProposal[];
  criticalPredictions: PredictionsResult["items"];
  recentMutations: AuditEntry[];
}): string {
  const lines = [
    `## Shift Brief — ${new Date().toISOString()}`,
    "",
    "### Health",
    `- Health score: ${snapshot.diagnostics.healthScore}/100`,
    `- Critical issues: ${snapshot.diagnostics.criticalIssues}`,
    `- Warnings: ${snapshot.diagnostics.warningIssues}`,
    "",
    `### Active Incidents (${openIncidents.length})`,
    ...toMarkdownList(
      openIncidents.map((incident) => `- [${incident.status}] ${incident.title} — started ${formatRelativeTime(incident.openedAt)}`),
    ),
    "",
    `### Pending Remediations (${pendingRemediations.length})`,
    ...toMarkdownList(
      pendingRemediations.map(
        (item) => `- [${item.riskLevel}] ${item.resource || "unknown resource"} — ${item.reason || "No reason provided"}`,
      ),
    ),
    "",
    `### Top Predictions (${criticalPredictions.length})`,
    ...toMarkdownList(
      criticalPredictions.map((item) => {
        const resource = item.namespace ? `${item.namespace}/${item.resource}` : item.resource;
        return `- [${item.confidence}%] ${resource} — ${item.summary}`;
      }),
    ),
    "",
    `### Recent Audit Activity (${recentMutations.length} entries)`,
    ...toMarkdownList(
      recentMutations.map((entry) => {
        const actor = entry.user?.trim() || "unknown";
        const route = entry.route?.trim() || entry.path;
        return `- ${actor} · ${route} · ${entry.status} · ${formatIsoTimestamp(entry.timestamp)}`;
      }),
    ),
  ];

  return lines.join("\n");
}

function toMarkdownList(entries: string[]): string[] {
  return entries.length > 0 ? entries : ["- None"];
}

function formatRelativeTime(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return "unknown";
  }

  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }

  const diffMs = date.getTime() - Date.now();
  const diffMinutes = Math.round(diffMs / 60000);
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

  if (Math.abs(diffMinutes) < 60) {
    return formatter.format(diffMinutes, "minute");
  }

  const diffHours = Math.round(diffMinutes / 60);
  if (Math.abs(diffHours) < 24) {
    return formatter.format(diffHours, "hour");
  }

  const diffDays = Math.round(diffHours / 24);
  return formatter.format(diffDays, "day");
}

function formatIsoTimestamp(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return "-";
  }

  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }

  return date.toISOString();
}
