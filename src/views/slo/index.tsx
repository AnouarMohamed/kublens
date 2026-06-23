import { useCallback, useMemo } from "react";
import { useAsyncResource } from "../../app/hooks/useAsyncResource";
import { KpiStrip } from "../../components/KpiStrip";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import type { SLOObjective, SLOOverview, SLOStatus } from "../../types";

export default function SLOView() {
  const { can } = useAuthSession();
  const canRead = can("read");

  const loadSLOOverview = useCallback((signal: AbortSignal) => {
    return api.getSLOOverview(signal);
  }, []);

  const {
    data: overview,
    isLoading,
    error,
    load,
  } = useAsyncResource<SLOOverview | null>({
    loader: loadSLOOverview,
    fallbackError: "Failed to load slo overview",
    initialData: null,
    enabled: canRead,
    disabledData: null,
    disabledError: "Authenticate to view slo data.",
  });

  const kpiItems = useMemo(() => {
    if (!overview) {
      return [
        { label: "Healthy", value: "0" },
        { label: "At Risk", value: "0" },
        { label: "Breached", value: "0" },
        { label: "Avg Budget", value: "0.0%" },
      ];
    }

    const avgBudget =
      overview.objectives.length === 0
        ? 0
        : overview.objectives.reduce((sum, item) => sum + item.budgetRemainingPercent, 0) / overview.objectives.length;

    return [
      { label: "Healthy", value: String(overview.healthyObjectives), tone: "healthy" as const },
      { label: "At Risk", value: String(overview.atRiskObjectives), tone: "warning" as const },
      { label: "Breached", value: String(overview.breachedObjectives), tone: "critical" as const },
      {
        label: "Avg Budget",
        value: `${avgBudget.toFixed(1)}%`,
        tone: avgBudget >= 70 ? ("healthy" as const) : avgBudget >= 40 ? ("warning" as const) : ("critical" as const),
      },
    ];
  }, [overview]);

  return (
    <div className="space-y-4">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">SLO Center</h2>
          <p className="mt-1 text-sm text-zinc-400">
            Error-budget posture across request health, cluster readiness, and operational execution.
          </p>
        </div>
        <button onClick={() => void load()} disabled={isLoading} className="btn">
          {isLoading ? "Refreshing..." : "Refresh"}
        </button>
      </header>

      {error && <div className="surface px-3 py-2 text-sm text-zinc-200">{error}</div>}

      <KpiStrip items={kpiItems} />

      {overview && (
        <>
          <section className="surface p-4">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">Control Summary</p>
                <p className="mt-2 text-lg text-zinc-100">{overview.summary}</p>
                <p className="mt-1 text-xs text-zinc-500">Generated {formatTimestamp(overview.generatedAt)}</p>
              </div>
              <div className="flex flex-wrap gap-2">
                {overview.alerts.map((alert) => (
                  <span key={alert} className="border border-zinc-700 bg-zinc-950 px-3 py-1 text-xs text-zinc-300">
                    {alert}
                  </span>
                ))}
              </div>
            </div>
          </section>

          <section className="grid gap-4 xl:grid-cols-2">
            {overview.objectives.map((objective) => (
              <SLOCard key={objective.id} objective={objective} />
            ))}
          </section>
        </>
      )}
    </div>
  );
}

function SLOCard({ objective }: { objective: SLOObjective }) {
  return (
    <article className="surface p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">{objective.category}</p>
          <h3 className="mt-2 text-lg font-semibold text-zinc-100">{objective.name}</h3>
        </div>
        <span className={`border px-2 py-1 text-[11px] uppercase ${statusBadgeClass(objective.status)}`}>
          {objective.status.replaceAll("_", " ")}
        </span>
      </div>

      <p className="mt-3 text-sm leading-relaxed text-zinc-400">{objective.summary}</p>

      <div className="mt-4 grid gap-3 md:grid-cols-4">
        <MetricCell label="Current" value={objective.currentValue} />
        <MetricCell label="Target" value={objective.targetValue} />
        <MetricCell label="Budget Left" value={`${objective.budgetRemainingPercent.toFixed(1)}%`} />
        <MetricCell label="Burn Rate" value={`${objective.burnRate.toFixed(1)}x`} />
      </div>

      <div className="mt-4">
        <div className="flex items-center justify-between gap-2 text-[11px] uppercase tracking-[0.18em] text-zinc-500">
          <span>Error Budget Used</span>
          <span>{objective.errorBudgetUsedPercent.toFixed(1)}%</span>
        </div>
        <div className="mt-2 h-1.5 bg-zinc-800">
          <div
            className={`h-full ${statusFillClass(objective.status)}`}
            style={{ width: `${Math.max(2, Math.min(100, objective.errorBudgetUsedPercent))}%` }}
          />
        </div>
        <p className="mt-2 text-xs text-zinc-500">Window: {objective.window}</p>
      </div>

      <div className="mt-4 border-t border-zinc-700 pt-4">
        <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">Signals</p>
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          {objective.signals.map((signal) => (
            <div key={`${objective.id}-${signal.label}`} className="border border-zinc-700 bg-zinc-950 px-3 py-2">
              <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{signal.label}</p>
              <p className={`mt-1 text-sm ${signalToneClass(signal.tone)}`}>{signal.value}</p>
            </div>
          ))}
        </div>
      </div>
    </article>
  );
}

function MetricCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-zinc-700 bg-zinc-950 px-3 py-2">
      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{label}</p>
      <p className="mt-1 text-lg font-semibold text-zinc-100">{value}</p>
    </div>
  );
}

function statusBadgeClass(status: SLOStatus): string {
  if (status === "healthy") {
    return "border-[var(--accent)] bg-[var(--accent-dim)] text-zinc-100";
  }
  if (status === "at_risk") {
    return "border-[var(--amber)]/50 bg-[var(--amber)]/10 text-[var(--amber)]";
  }
  return "border-[var(--red)]/45 bg-[var(--red)]/10 text-[var(--red)]";
}

function statusFillClass(status: SLOStatus): string {
  if (status === "healthy") {
    return "bg-[var(--accent)]";
  }
  if (status === "at_risk") {
    return "bg-[var(--amber)]";
  }
  return "bg-[var(--red)]";
}

function signalToneClass(tone: string): string {
  if (tone === "healthy") {
    return "text-zinc-100";
  }
  if (tone === "warning") {
    return "text-[var(--amber)]";
  }
  if (tone === "critical") {
    return "text-[var(--red)]";
  }
  return "text-zinc-300";
}

function formatTimestamp(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}
