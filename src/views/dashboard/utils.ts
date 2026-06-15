import { ACCENT, AMBER, MUTED, RED } from "./constants";
import type { ApiMetricsSnapshot, ClusterStats, DiagnosticsResult, K8sEvent, Node, Pod } from "../../types";

export function buildPrioritizedIssues(diagnostics: DiagnosticsResult | null) {
  if (!diagnostics) {
    return [];
  }

  const rank = { critical: 0, warning: 1, info: 2 } as const;
  return [...diagnostics.issues].sort((a, b) => rank[a.severity] - rank[b.severity]).slice(0, 8);
}

export function buildPodMixData(stats: ClusterStats | null): Array<{ name: string; value: number; color: string }> {
  if (!stats || stats.pods.total === 0) {
    return [];
  }

  const succeeded = Math.max(stats.pods.total - stats.pods.running - stats.pods.pending - stats.pods.failed, 0);
  return [
    { name: "Running", value: stats.pods.running, color: ACCENT },
    { name: "Pending", value: stats.pods.pending, color: AMBER },
    { name: "Failed", value: stats.pods.failed, color: RED },
    { name: "Succeeded", value: succeeded, color: MUTED },
  ];
}

export function buildNodeUsageBars(nodes: Node[]): Array<{ name: string; cpu: number; memory: number }> {
  return nodes.slice(0, 8).map((node) => ({
    name: compactLabel(node.name),
    cpu: parsePercentNumber(node.cpuUsage),
    memory: parsePercentNumber(node.memUsage),
  }));
}

export function buildNodeCPUTrend(nodes: Node[]): Array<{ time: string; value: number }> {
  const pointCount = nodes.reduce((max, node) => Math.max(max, node.cpuHistory?.length ?? 0), 0);
  if (pointCount === 0) {
    return [];
  }

  const rows: Array<{ time: string; value: number }> = [];
  for (let i = 0; i < pointCount; i++) {
    let total = 0;
    let count = 0;
    let label = "";

    for (const node of nodes) {
      const point = node.cpuHistory?.[i];
      if (!point) {
        continue;
      }
      total += point.value;
      count++;
      if (!label && point.time) {
        label = point.time;
      }
    }

    if (count > 0) {
      rows.push({
        time: label || `T${i + 1}`,
        value: Number((total / count).toFixed(2)),
      });
    }
  }

  return rows;
}

export function buildRestartHotspots(pods: Pod[]): Array<{ name: string; restarts: number; color: string }> {
  return [...pods]
    .sort((a, b) => b.restarts - a.restarts)
    .slice(0, 7)
    .map((pod) => ({
      name: compactLabel(pod.name),
      restarts: pod.restarts,
      color: restartSeverityColor(pod.restarts),
    }));
}

export function buildEventReasonBars(events: K8sEvent[]): Array<{ name: string; count: number }> {
  const byReason = new Map<string, number>();
  for (const event of events) {
    const reason = (event.reason || "Unknown").trim() || "Unknown";
    byReason.set(reason, (byReason.get(reason) ?? 0) + (event.count && event.count > 0 ? event.count : 1));
  }

  return [...byReason.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([name, count]) => ({ name: compactLabel(name), count }));
}

export function buildAPIStatusMix(metrics: ApiMetricsSnapshot | null) {
  if (!metrics) {
    return { ok: 0, redirect: 0, clientError: 0, serverError: 0, total: 0 };
  }

  const ok = metrics.routes.reduce((sum, route) => sum + route.status2xx, 0);
  const redirect = metrics.routes.reduce((sum, route) => sum + route.status3xx, 0);
  const clientError = metrics.routes.reduce((sum, route) => sum + route.status4xx, 0);
  const serverError = metrics.routes.reduce((sum, route) => sum + route.status5xx, 0);
  return { ok, redirect, clientError, serverError, total: ok + redirect + clientError + serverError };
}

export function parsePercentNumber(value: string): number {
  const numeric = Number.parseFloat(value.replace("%", ""));
  if (!Number.isFinite(numeric)) {
    return 0;
  }
  return Number(Math.max(0, Math.min(100, numeric)).toFixed(2));
}

export function coerceNumber(value: number | string | readonly (number | string)[] | undefined): number {
  if (Array.isArray(value)) {
    return coerceNumber(value[0]);
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : 0;
  }
  if (typeof value === "string") {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

export function percentage(part: number, whole: number): number {
  if (!Number.isFinite(part) || !Number.isFinite(whole) || whole <= 0) {
    return 0;
  }
  return Number(((part / whole) * 100).toFixed(2));
}

export function compactLabel(value: string): string {
  if (value.length <= 20) {
    return value;
  }
  return `${value.slice(0, 17)}...`;
}

export function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

export function riskColor(value: number): string {
  if (value > 20) {
    return RED;
  }
  if (value > 5) {
    return AMBER;
  }
  return ACCENT;
}

export function restartSeverityColor(restarts: number): string {
  if (restarts > 10) {
    return RED;
  }
  if (restarts >= 3) {
    return AMBER;
  }
  return "#666666";
}

export function restartCountColorClass(restarts: number): string {
  if (restarts > 10) {
    return "text-[#ff4444]";
  }
  if (restarts >= 3) {
    return "text-[#f59e0b]";
  }
  return "text-[#666666]";
}

export function normalizeLifecycleRows(
  data: Array<{ name: string; value: number; color: string }>,
): Array<{ name: string; value: number; color: string }> {
  const defaults = [
    { name: "Running", color: ACCENT },
    { name: "Pending", color: AMBER },
    { name: "Failed", color: RED },
    { name: "Succeeded", color: MUTED },
  ];

  return defaults.map((item) => ({
    name: item.name,
    color: item.color,
    value: data.find((row) => row.name.toLowerCase() === item.name.toLowerCase())?.value ?? 0,
  }));
}

export function lifecycleCount(data: Array<{ name: string; value: number }>, target: string): number {
  return data.find((row) => row.name.toLowerCase() === target)?.value ?? 0;
}
