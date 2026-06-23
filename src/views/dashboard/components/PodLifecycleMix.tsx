import { EmptyChart, LifecycleMiniStat } from "./DashboardPrimitives";
import { lifecycleCount, normalizeLifecycleRows, percentage } from "../utils";

function lifecycleSignalTone(atRiskPercent: number): string {
  if (atRiskPercent >= 30) {
    return "text-[#ff4444]";
  }
  if (atRiskPercent >= 10) {
    return "text-[#f59e0b]";
  }
  return "text-[#e8e8e8]";
}

export function PodLifecycleMix({ data }: { data: Array<{ name: string; value: number; color: string }> }) {
  const total = data.reduce((sum, row) => sum + row.value, 0);
  if (total === 0) {
    return <EmptyChart message="No pod lifecycle data." />;
  }

  const rows = normalizeLifecycleRows(data);
  const runningCount = lifecycleCount(rows, "running");
  const pendingCount = lifecycleCount(rows, "pending");
  const failedCount = lifecycleCount(rows, "failed");
  const succeededCount = lifecycleCount(rows, "succeeded");
  const healthyPercent = percentage(runningCount, total);
  const atRiskCount = pendingCount + failedCount;
  const atRiskPercent = percentage(atRiskCount, total);
  const dominant = [...rows].sort((a, b) => b.value - a.value)[0];

  return (
    <div className="h-[346px] flex flex-col py-1">
      <div className="rounded-md border border-zinc-700 bg-zinc-900/60 px-3 py-2">
        <div className="flex items-center justify-between gap-3">
          <p className="text-[10px] font-mono uppercase tracking-[0.15em] text-[#555555]">Live Distribution</p>
          <p className="text-[11px] font-mono text-[#888888]">
            Dominant: <span className="text-[#e8e8e8]">{dominant.name}</span>{" "}
            {percentage(dominant.value, total).toFixed(0)}%
          </p>
        </div>
        <div className="mt-2 h-2 bg-[#1f1f1f] overflow-hidden flex">
          {rows.map((row) => {
            const pct = percentage(row.value, total);
            return (
              <div
                key={`mix-segment-${row.name}`}
                className="h-full transition-all duration-300"
                style={{ width: `${pct}%`, backgroundColor: row.color, minWidth: row.value > 0 ? "2px" : "0px" }}
              />
            );
          })}
        </div>
      </div>

      <div className="mt-3 grid grid-cols-1 sm:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] gap-3 flex-1 min-h-0">
        <div className="space-y-2">
          {rows.map((row) => {
            const pct = percentage(row.value, total);
            return (
              <article key={row.name} className="rounded-md border border-zinc-800 bg-zinc-900/45 px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-[11px] font-mono text-[#666666] flex items-center gap-2">
                    <span
                      className="inline-block h-2 w-2 rounded-full border"
                      style={{
                        backgroundColor: row.value > 0 ? row.color : "transparent",
                        borderColor: row.value > 0 ? row.color : "#3f3f46",
                      }}
                    />
                    {row.name}
                  </p>
                  <p className="text-xs font-mono">
                    <span className="font-semibold text-[#e8e8e8]">{row.value}</span>
                    <span className="text-[#555555] ml-2">{pct.toFixed(0)}%</span>
                  </p>
                </div>
                <div className="mt-1.5 h-1.5 bg-[#1f1f1f] overflow-hidden">
                  <div
                    className="h-full transition-all duration-300"
                    style={{ width: `${pct}%`, backgroundColor: row.color }}
                  />
                </div>
              </article>
            );
          })}
        </div>

        <div className="grid grid-cols-2 gap-2 content-start">
          <LifecycleMiniStat label="Total" value={String(total)} tone="neutral" />
          <LifecycleMiniStat label="Healthy" value={`${healthyPercent.toFixed(0)}%`} tone="good" />
          <LifecycleMiniStat label="At Risk" value={String(atRiskCount)} tone="bad" />
          <LifecycleMiniStat
            label="Succeeded"
            value={String(succeededCount)}
            tone={succeededCount > 0 ? "good" : "neutral"}
          />
          <div className="col-span-2 rounded-md border border-zinc-800 bg-zinc-900/45 px-2.5 py-2">
            <p className="text-[10px] font-mono uppercase tracking-[0.12em] text-[#555555]">Operational Signal</p>
            <p className={`mt-1 text-xs font-mono ${lifecycleSignalTone(atRiskPercent)}`}>
              {atRiskPercent >= 30
                ? "High pod lifecycle risk detected."
                : atRiskPercent >= 10
                  ? "Watch pending and failed pod pressure."
                  : "Lifecycle mix is within normal operating bounds."}
            </p>
          </div>
        </div>
      </div>

      <div className="mt-3 pt-3 border-t border-[#1f1f1f] grid grid-cols-3 gap-4">
        <div>
          <p className="text-[10px] font-mono uppercase tracking-[0.15em] text-[#444444]">Running</p>
          <p className="mt-1 text-lg font-mono font-semibold text-[#e8e8e8]">{runningCount}</p>
        </div>
        <div>
          <p className="text-[10px] font-mono uppercase tracking-[0.15em] text-[#444444]">Pending</p>
          <p className="mt-1 text-lg font-mono font-semibold text-[#f59e0b]">{pendingCount}</p>
        </div>
        <div>
          <p className="text-[10px] font-mono uppercase tracking-[0.15em] text-[#444444]">Failed</p>
          <p className="mt-1 text-lg font-mono font-semibold text-[#ff4444]">{failedCount}</p>
        </div>
      </div>
    </div>
  );
}
