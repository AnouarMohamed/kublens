import type { ReactNode } from "react";
import { AMBER, BLUE, RED } from "../constants";

export function RiskRail({ label, value, onClick }: { label: string; value: number; onClick?: () => void }) {
  const color = riskRailColor(value);
  const baseClass = "rounded-xl border border-zinc-700 bg-zinc-900 p-4";

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`${baseClass} text-left transition-colors hover:border-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]`}
        title={`Open related view for ${label}`}
      >
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">{label}</p>
          <p className="text-sm font-semibold text-zinc-100">{value.toFixed(1)}%</p>
        </div>
        <div className="mt-3 h-1 rounded-none bg-zinc-700 overflow-hidden">
          <div
            className="h-full rounded-none"
            style={{ width: `${Math.max(0, Math.min(100, value))}%`, backgroundColor: color }}
          />
        </div>
      </button>
    );
  }

  return (
    <div className={baseClass}>
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-zinc-500">{label}</p>
        <p className="text-sm font-semibold text-zinc-100">{value.toFixed(1)}%</p>
      </div>
      <div className="mt-3 h-1 rounded-none bg-zinc-700 overflow-hidden">
        <div
          className="h-full rounded-none"
          style={{ width: `${Math.max(0, Math.min(100, value))}%`, backgroundColor: color }}
        />
      </div>
    </div>
  );
}

export function ChartCard({
  title,
  subtitle,
  children,
  onDrilldown,
  drilldownLabel,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
  onDrilldown?: () => void;
  drilldownLabel?: string;
}) {
  return (
    <div className="surface p-5">
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-sm font-semibold text-zinc-100">{title}</h3>
        {onDrilldown && (
          <button type="button" onClick={onDrilldown} className="btn-sm whitespace-nowrap">
            {drilldownLabel ?? "Open view"}
          </button>
        )}
      </div>
      <p className="text-xs text-zinc-400 mt-1">{subtitle}</p>
      <div className="mt-4">{children}</div>
    </div>
  );
}

export function EmptyChart({ message }: { message: string }) {
  return (
    <div className="h-[320px] flex items-center justify-center rounded-md border border-dashed border-zinc-700 text-sm text-zinc-500">
      {message}
    </div>
  );
}

export function StackBar({
  label,
  value,
  total,
  color,
}: {
  label: string;
  value: number;
  total: number;
  color: string;
}) {
  const ratio = total > 0 ? (value / total) * 100 : 0;
  return (
    <div>
      <div className="flex items-center justify-between gap-2 text-xs">
        <p className="font-semibold text-zinc-300">{label}</p>
        <p className="text-zinc-500">
          {value} ({ratio.toFixed(1)}%)
        </p>
      </div>
      <div className="mt-1.5 h-1.5 rounded-none bg-zinc-700 overflow-hidden">
        <div
          className="h-full rounded-none"
          style={{ width: `${Math.max(0, Math.min(100, ratio))}%`, backgroundColor: color }}
        />
      </div>
    </div>
  );
}

export function SnapshotRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-md border border-zinc-700 bg-zinc-900/50 px-3 py-2">
      <span className="text-zinc-500">{label}</span>
      <span className="font-semibold text-zinc-100">{value}</span>
    </div>
  );
}

export function FindingCard({
  severity,
  title,
  resource,
  details,
  recommendation,
}: {
  severity: "critical" | "warning" | "info";
  title: string;
  resource?: string;
  details: string;
  recommendation: string;
}) {
  const severityColor = severity === "critical" ? RED : severity === "warning" ? AMBER : BLUE;

  return (
    <article className="rounded-md border border-zinc-700 bg-zinc-900/50 px-3 py-2">
      <div className="flex items-center gap-2">
        <span className="text-[10px] font-mono font-semibold uppercase" style={{ color: severityColor }}>
          {severity}
        </span>
        <p className="text-xs font-mono font-semibold text-[#e8e8e8]">{title}</p>
        {resource && <span className="text-[11px] font-mono text-[#444444]">{resource}</span>}
      </div>
      <p className="mt-1 text-xs font-mono text-[#666666] leading-relaxed">{details}</p>
      <p className="mt-1 text-[11px] font-mono text-[#666666]">-&gt; {recommendation}</p>
    </article>
  );
}

export function LifecycleMiniStat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "neutral" | "good" | "bad" | "info";
}) {
  const valueClass = tone === "bad" ? "text-[#ff4444]" : tone === "info" ? "text-[#3b82f6]" : "text-[#e8e8e8]";

  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900/45 px-2.5 py-2">
      <p className="text-[10px] font-mono uppercase tracking-[0.12em] text-[#555555]">{label}</p>
      <p className={`mt-1 text-sm font-mono font-semibold ${valueClass}`}>{value}</p>
    </div>
  );
}

function riskRailColor(value: number): string {
  if (value > 20) {
    return RED;
  }
  if (value > 5) {
    return AMBER;
  }
  return "#e8e8e8";
}
