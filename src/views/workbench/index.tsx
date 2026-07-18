import { ExternalLink, FileText, Network, RefreshCw, ShieldCheck, TrendingUp, Wrench } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { navigateToView } from "../../app/viewNavigation";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import type {
  AuditEntry,
  DiagnosticsResult,
  GhostSimulationListResponse,
  HealthStatus,
  IncidentPrediction,
  PredictionsResult,
  RemediationProposal,
  RuntimeStatus,
} from "../../types";

interface LoadResult<T> {
  data: T | null;
  error: string | null;
}

interface WorkbenchSnapshot {
  runtime: LoadResult<RuntimeStatus>;
  enterprise: LoadResult<HealthStatus>;
  diagnostics: LoadResult<DiagnosticsResult>;
  predictions: LoadResult<PredictionsResult>;
  remediations: LoadResult<RemediationProposal[]>;
  ghostRuns: LoadResult<GhostSimulationListResponse>;
  audit: LoadResult<AuditEntry[]>;
}

export default function IncidentWorkbench() {
  const { can, isLoading: authLoading } = useAuthSession();
  const canRead = can("read");
  const [snapshot, setSnapshot] = useState<WorkbenchSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [lastLoadedAt, setLastLoadedAt] = useState<string>("");

  const load = useCallback(async () => {
    if (authLoading) {
      return;
    }
    if (!canRead) {
      setSnapshot(null);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    const [runtime, enterprise, diagnostics, predictions, remediations, ghostRuns, audit] = await Promise.all([
      capture(api.getRuntimeStatus()),
      capture(api.getEnterpriseReadiness()),
      capture(api.getDiagnostics()),
      capture(api.getPredictions()),
      capture(api.listRemediation()),
      capture(api.listGhostSimulations(5)),
      capture(api.getAuditLog(20).then((response) => response.items)),
    ]);

    setSnapshot({
      runtime,
      enterprise,
      diagnostics,
      predictions,
      remediations,
      ghostRuns,
      audit,
    });
    setLastLoadedAt(new Date().toISOString());
    setIsLoading(false);
  }, [authLoading, canRead]);

  useEffect(() => {
    void load();
  }, [load]);

  const topPredictions = useMemo(() => snapshot?.predictions.data?.items.slice(0, 5) ?? [], [snapshot]);
  const highRiskCount = useMemo(
    () => snapshot?.predictions.data?.items.filter((item) => item.riskScore >= 80).length ?? 0,
    [snapshot],
  );
  const queuedRemediations = useMemo(
    () =>
      (snapshot?.remediations.data ?? [])
        .filter((item) => item.status === "proposed" || item.status === "approved")
        .sort(compareRemediationPriority)
        .slice(0, 5),
    [snapshot],
  );
  const latestGhostRun = snapshot?.ghostRuns.data?.items[0] ?? null;
  const latestAudit = snapshot?.audit.data?.[0] ?? null;
  const blockedChecks = snapshot?.enterprise.data?.checks.filter((check) => !check.ok) ?? [];
  const loadErrors = useMemo(() => collectErrors(snapshot), [snapshot]);

  return (
    <div className="space-y-5">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight text-zinc-100">Incident Workbench</h2>
          <p className="mt-1 text-sm text-zinc-500">
            Detect, simulate, remediate, and audit the current operational queue.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => void load()}
            className="btn-sm inline-flex items-center gap-2"
            disabled={isLoading || !canRead}
          >
            <RefreshCw size={14} />
            {isLoading ? "Refreshing" : "Refresh"}
          </button>
          <WorkbenchNavButton label="Predictions" view="predictions" icon={<TrendingUp size={14} />} />
          <WorkbenchNavButton label="Ghost" view="ghost" icon={<Network size={14} />} />
          <WorkbenchNavButton label="Remediation" view="remediation" icon={<Wrench size={14} />} />
          <WorkbenchNavButton label="Brief" view="shiftbrief" icon={<FileText size={14} />} />
        </div>
      </header>

      {!canRead && !authLoading && (
        <StatusBanner tone="warn" text="Authenticate with read access to load workbench data." />
      )}
      {loadErrors.length > 0 && (
        <StatusBanner tone="warn" text={`Partial data loaded. Failed: ${loadErrors.join(", ")}.`} />
      )}

      <section className="kpi-strip">
        <KpiCell
          label="Enterprise"
          value={snapshot?.enterprise.data?.status ?? "unknown"}
          tone={readinessTone(snapshot?.enterprise.data?.status)}
          note={`${blockedChecks.length} blocked checks`}
        />
        <KpiCell
          label="Predictor"
          value={snapshot?.runtime.data?.predictorMode ?? "unknown"}
          tone={snapshot?.runtime.data?.predictorHealthy ? "good" : "warn"}
          note={snapshot?.runtime.data?.predictorEnabled ? "external scorer" : "fallback only"}
        />
        <KpiCell
          label="Ghost"
          value={snapshot?.runtime.data?.ghostHealthy ? "ready" : "degraded"}
          tone={snapshot?.runtime.data?.ghostHealthy ? "good" : "warn"}
          note={latestGhostRun ? `${latestGhostRun.result.confidence}% last run` : "no stored run"}
        />
        <KpiCell
          label="High Risk"
          value={String(highRiskCount)}
          tone={highRiskCount > 0 ? "bad" : "good"}
          note={`${topPredictions.length} visible predictions`}
        />
        <KpiCell
          label="Audit"
          value={latestAudit?.hash ? "hashed" : "pending"}
          tone={latestAudit?.hash ? "good" : "neutral"}
          note={latestAudit ? `${latestAudit.status} ${latestAudit.path}` : "no entries"}
        />
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(340px,0.65fr)]">
        <div className="surface p-4">
          <SectionHead
            eyebrow="Detect"
            title="Risk Queue"
            actionLabel="Open predictions"
            actionView="predictions"
            icon={<TrendingUp size={14} />}
          />
          <div className="mt-3 divide-y divide-zinc-800 border border-zinc-800">
            {topPredictions.map((item) => (
              <PredictionRow key={item.id} item={item} />
            ))}
            {topPredictions.length === 0 && (
              <EmptyRow
                text={isLoading ? "Loading predictions..." : "No predicted incidents above the display floor."}
              />
            )}
          </div>
        </div>

        <div className="surface p-4">
          <SectionHead
            eyebrow="Readiness"
            title="Enterprise Posture"
            actionLabel="Open audit"
            actionView="audit"
            icon={<ShieldCheck size={14} />}
          />
          <div className="mt-3 space-y-2">
            {(snapshot?.enterprise.data?.checks ?? []).slice(0, 7).map((check) => (
              <div key={check.name} className="flex items-start justify-between gap-3 border border-zinc-800 px-3 py-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-zinc-100">{check.name}</p>
                  <p className="mt-1 line-clamp-2 text-xs text-zinc-500">{check.message}</p>
                </div>
                <span className={`shrink-0 border px-2 py-0.5 text-[11px] ${check.ok ? goodBadge : warnBadge}`}>
                  {check.ok ? "ok" : "blocked"}
                </span>
              </div>
            ))}
            {(snapshot?.enterprise.data?.checks.length ?? 0) === 0 && (
              <EmptyRow text={isLoading ? "Loading readiness..." : "Enterprise readiness is unavailable."} />
            )}
          </div>
        </div>
      </section>

      <section className="grid gap-4 xl:grid-cols-3">
        <WorkbenchPanel
          eyebrow="Simulate"
          title="Latest Ghost Run"
          actionLabel="Open Ghost"
          actionView="ghost"
          icon={<Network size={14} />}
        >
          {latestGhostRun ? (
            <div className="space-y-3">
              <div className="grid grid-cols-3 border border-zinc-800 text-xs">
                <MetricBlock label="Confidence" value={`${latestGhostRun.result.confidence}%`} />
                <MetricBlock label="Engine" value={latestGhostRun.result.engine} />
                <MetricBlock label="Frames" value={String(latestGhostRun.result.frames.length)} />
              </div>
              <p className="text-sm text-zinc-300">{latestGhostRun.result.verdict.summary}</p>
              <p className="text-xs text-zinc-500">
                {latestGhostRun.result.limitations[0] ?? "No limitation recorded."}
              </p>
            </div>
          ) : (
            <EmptyRow text={isLoading ? "Loading simulations..." : "No stored Ghost simulation yet."} />
          )}
        </WorkbenchPanel>

        <WorkbenchPanel
          eyebrow="Remediate"
          title="Governed Actions"
          actionLabel="Open remediation"
          actionView="remediation"
          icon={<Wrench size={14} />}
        >
          <div className="divide-y divide-zinc-800 border border-zinc-800">
            {queuedRemediations.map((item) => (
              <div key={item.id} className="px-3 py-2">
                <div className="flex items-start justify-between gap-3">
                  <p className="min-w-0 truncate text-sm font-medium text-zinc-100">{item.resource}</p>
                  <span className={`border px-2 py-0.5 text-[11px] ${riskBadgeClass(item.riskLevel)}`}>
                    {item.riskLevel}
                  </span>
                </div>
                <p className="mt-1 text-xs text-zinc-500">
                  {item.status} | {item.reason}
                </p>
              </div>
            ))}
            {queuedRemediations.length === 0 && (
              <EmptyRow text={isLoading ? "Loading remediation..." : "No proposed or approved remediation."} />
            )}
          </div>
        </WorkbenchPanel>

        <WorkbenchPanel
          eyebrow="Audit"
          title="Evidence Trail"
          actionLabel="Open audit"
          actionView="audit"
          icon={<ShieldCheck size={14} />}
        >
          <div className="space-y-3">
            <div className="grid grid-cols-2 border border-zinc-800 text-xs">
              <MetricBlock label="Entries" value={String(snapshot?.audit.data?.length ?? 0)} />
              <MetricBlock label="Latest Hash" value={latestAudit?.hash ? shortHash(latestAudit.hash) : "none"} />
            </div>
            {latestAudit ? (
              <p className="text-sm text-zinc-300">
                {latestAudit.method} {latestAudit.path} returned {latestAudit.status}.
              </p>
            ) : (
              <EmptyRow text={isLoading ? "Loading audit..." : "No audit entries returned."} />
            )}
            <p className="text-xs text-zinc-500">Last loaded {lastLoadedAt ? formatTime(lastLoadedAt) : "not yet"}</p>
          </div>
        </WorkbenchPanel>
      </section>
    </div>
  );
}

async function capture<T>(request: Promise<T>): Promise<LoadResult<T>> {
  try {
    return { data: await request, error: null };
  } catch (err) {
    return { data: null, error: err instanceof Error ? err.message : "request failed" };
  }
}

function collectErrors(snapshot: WorkbenchSnapshot | null): string[] {
  if (!snapshot) {
    return [];
  }
  return Object.entries(snapshot)
    .filter(([, value]) => value.error)
    .map(([key]) => key);
}

function compareRemediationPriority(a: RemediationProposal, b: RemediationProposal): number {
  return remediationRiskRank(b.riskLevel) - remediationRiskRank(a.riskLevel);
}

function remediationRiskRank(value: string): number {
  const normalized = value.trim().toLowerCase();
  if (normalized === "critical") {
    return 4;
  }
  if (normalized === "high") {
    return 3;
  }
  if (normalized === "medium") {
    return 2;
  }
  return 1;
}

function readinessTone(status?: string): KpiTone {
  if (status === "ok") {
    return "good";
  }
  if (status === "not-ready" || status === "degraded") {
    return "warn";
  }
  return "neutral";
}

function riskBadgeClass(risk: string): string {
  const normalized = risk.trim().toLowerCase();
  if (normalized === "critical" || normalized === "high") {
    return badBadge;
  }
  if (normalized === "medium") {
    return warnBadge;
  }
  return neutralBadge;
}

function predictionSignalText(item: IncidentPrediction): string {
  const signals = item.signals ?? [];
  const mlSignal = signals.find((signal) => signal.key === "mlRisk" || signal.key === "mlShadowRisk");
  const firstSignal = mlSignal ?? signals[0];
  if (!firstSignal) {
    return "no signal";
  }
  return `${firstSignal.key}: ${firstSignal.value}`;
}

function formatTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function shortHash(value: string): string {
  return value.length > 12 ? value.slice(0, 12) : value;
}

type KpiTone = "good" | "warn" | "bad" | "neutral";

function KpiCell({ label, value, note, tone }: { label: string; value: string; note: string; tone: KpiTone }) {
  const valueClass =
    tone === "good"
      ? "kpi-value-healthy"
      : tone === "warn"
        ? "kpi-value-warning"
        : tone === "bad"
          ? "kpi-value-critical"
          : "";
  return (
    <div className="kpi min-w-0">
      <p className="kpi-label">{label}</p>
      <p className={`kpi-value truncate text-lg ${valueClass}`}>{value}</p>
      <p className="mt-1 truncate text-xs text-zinc-500">{note}</p>
    </div>
  );
}

function PredictionRow({ item }: { item: IncidentPrediction }) {
  return (
    <div className="grid gap-3 px-3 py-3 md:grid-cols-[86px_minmax(0,1fr)_120px]">
      <div>
        <p className={`inline-flex border px-2 py-1 text-xs font-semibold ${riskScoreClass(item.riskScore)}`}>
          {item.riskScore}
        </p>
        <p className="mt-1 text-[11px] text-zinc-500">{item.confidence}% confidence</p>
      </div>
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-zinc-100">
          {item.resourceKind}: {item.namespace ? `${item.namespace}/` : ""}
          {item.resource}
        </p>
        <p className="mt-1 line-clamp-2 text-xs text-zinc-400">{item.summary}</p>
        <p className="mt-1 truncate text-[11px] text-zinc-500">{predictionSignalText(item)}</p>
      </div>
      <button
        type="button"
        onClick={() => navigateToView("diagnostics")}
        className="btn-sm inline-flex h-8 items-center justify-center gap-2 self-start"
      >
        <ExternalLink size={13} />
        Explain
      </button>
    </div>
  );
}

function riskScoreClass(score: number): string {
  if (score >= 80) {
    return badBadge;
  }
  if (score >= 60) {
    return warnBadge;
  }
  return goodBadge;
}

function SectionHead({
  eyebrow,
  title,
  actionLabel,
  actionView,
  icon,
}: {
  eyebrow: string;
  title: string;
  actionLabel: string;
  actionView: Parameters<typeof navigateToView>[0];
  icon: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p className="text-[10px] uppercase tracking-[0.22em] text-zinc-500">{eyebrow}</p>
        <h3 className="mt-1 text-sm font-semibold text-zinc-100">{title}</h3>
      </div>
      <WorkbenchNavButton label={actionLabel} view={actionView} icon={icon} />
    </div>
  );
}

function WorkbenchPanel({
  eyebrow,
  title,
  actionLabel,
  actionView,
  icon,
  children,
}: {
  eyebrow: string;
  title: string;
  actionLabel: string;
  actionView: Parameters<typeof navigateToView>[0];
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="surface p-4">
      <SectionHead eyebrow={eyebrow} title={title} actionLabel={actionLabel} actionView={actionView} icon={icon} />
      <div className="mt-3">{children}</div>
    </section>
  );
}

function WorkbenchNavButton({
  label,
  view,
  icon,
}: {
  label: string;
  view: Parameters<typeof navigateToView>[0];
  icon: ReactNode;
}) {
  return (
    <button type="button" onClick={() => navigateToView(view)} className="btn-sm inline-flex items-center gap-2">
      {icon}
      {label}
    </button>
  );
}

function MetricBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 border-l border-zinc-800 px-3 py-2 first:border-l-0">
      <p className="text-[10px] uppercase tracking-[0.18em] text-zinc-500">{label}</p>
      <p className="mt-1 truncate text-sm font-medium text-zinc-100">{value}</p>
    </div>
  );
}

function EmptyRow({ text }: { text: string }) {
  return <p className="px-3 py-6 text-center text-sm text-zinc-500">{text}</p>;
}

function StatusBanner({ tone, text }: { tone: "warn"; text: string }) {
  const toneClass = tone === "warn" ? "border-[var(--amber)]/45 bg-[var(--amber)]/10 text-zinc-100" : neutralBadge;
  return <div className={`border px-3 py-2 text-sm ${toneClass}`}>{text}</div>;
}

const goodBadge = "border-[#22c55e]/40 bg-[#22c55e]/12 text-[#bbf7d0]";
const warnBadge = "border-[var(--amber)]/45 bg-[var(--amber)]/12 text-[#fde68a]";
const badBadge = "border-[#ff4444]/40 bg-[#ff4444]/12 text-[#fecaca]";
const neutralBadge = "border-zinc-700 bg-zinc-900/70 text-zinc-300";
