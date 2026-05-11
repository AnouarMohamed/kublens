import { CHART_AMBER, CHART_BLUE, CHART_COLORS, CHART_GREEN, CHART_MAGENTA } from "./constants";
import type { ApiMetricsSnapshot, ClusterStats, Node, Pod } from "../../types";

export function buildPodLifecycleBars(
  stats: ClusterStats | null,
): Array<{ name: string; value: number; color: string }> {
  if (!stats) {
    return [];
  }

  const succeeded = Math.max(stats.pods.total - stats.pods.running - stats.pods.pending - stats.pods.failed, 0);
  return [
    { name: "Running", value: stats.pods.running, color: CHART_GREEN },
    { name: "Pending", value: stats.pods.pending, color: CHART_AMBER },
    { name: "Failed", value: stats.pods.failed, color: CHART_MAGENTA },
    { name: "Succeeded", value: succeeded, color: CHART_BLUE },
  ].filter((row) => row.value > 0);
}

export function buildNodeUtilizationBars(nodes: Node[]): Array<{ name: string; cpu: number; memory: number }> {
  return nodes.slice(0, 8).map((node) => ({
    name: compactRoute(node.name),
    cpu: parsePercentValue(node.cpuUsage),
    memory: parsePercentValue(node.memUsage),
  }));
}

export function buildNodeCPUTrend(nodes: Node[]): Array<{ time: string; value: number }> {
  const maxPoints = nodes.reduce((max, node) => Math.max(max, node.cpuHistory?.length ?? 0), 0);
  if (maxPoints === 0) {
    return [];
  }

  const rows: Array<{ time: string; value: number }> = [];
  for (let index = 0; index < maxPoints; index++) {
    let total = 0;
    let count = 0;
    let label = "";

    for (const node of nodes) {
      const point = node.cpuHistory?.[index];
      if (!point) {
        continue;
      }

      if (!label && point.time) {
        label = point.time;
      }
      total += Math.min(Math.max(point.value, 0), 100);
      count++;
    }

    if (count > 0) {
      rows.push({
        time: label || `T${index + 1}`,
        value: Number((total / count).toFixed(2)),
      });
    }
  }

  return rows;
}

export function buildRestartBands(pods: Pod[]): Array<{ name: string; value: number; color: string }> {
  let none = 0;
  let light = 0;
  let medium = 0;
  let heavy = 0;

  for (const pod of pods) {
    if (pod.restarts >= 10) {
      heavy++;
    } else if (pod.restarts >= 5) {
      medium++;
    } else if (pod.restarts >= 1) {
      light++;
    } else {
      none++;
    }
  }

  return [
    { name: "No Restarts", value: none, color: CHART_GREEN },
    { name: "1-4", value: light, color: CHART_BLUE },
    { name: "5-9", value: medium, color: CHART_AMBER },
    { name: "10+", value: heavy, color: CHART_MAGENTA },
  ].filter((row) => row.value > 0);
}

export function buildTopPodPressure(
  pods: Pod[],
): Array<{ name: string; score: number; cpuMilli: number; memMi: number; color: string }> {
  return pods
    .map((pod) => {
      const cpuMilli = parseCPUMilli(pod.cpu);
      const memMi = parseMemoryMi(pod.memory);
      const normalizedCPU = Math.min((cpuMilli / 1000) * 100, 100);
      const normalizedMemory = Math.min((memMi / 1024) * 100, 100);

      return {
        name: compactRoute(pod.name),
        score: Number((normalizedCPU * 0.6 + normalizedMemory * 0.4).toFixed(2)),
        cpuMilli,
        memMi,
      };
    })
    .sort((a, b) => b.score - a.score)
    .slice(0, 8)
    .map((row, index) => ({ ...row, color: CHART_COLORS[index % CHART_COLORS.length] }));
}

export function buildAPIStatusTotals(metrics: ApiMetricsSnapshot | null) {
  if (!metrics) {
    return { ok: 0, redirect: 0, clientError: 0, serverError: 0, total: 0 };
  }

  const ok = metrics.routes.reduce((sum, route) => sum + route.status2xx, 0);
  const redirect = metrics.routes.reduce((sum, route) => sum + route.status3xx, 0);
  const clientError = metrics.routes.reduce((sum, route) => sum + route.status4xx, 0);
  const serverError = metrics.routes.reduce((sum, route) => sum + route.status5xx, 0);
  const total = ok + redirect + clientError + serverError;

  return { ok, redirect, clientError, serverError, total };
}

export function buildAPIStatusStack(totals: {
  ok: number;
  redirect: number;
  clientError: number;
  serverError: number;
}) {
  if (totals.ok + totals.redirect + totals.clientError + totals.serverError === 0) {
    return [];
  }

  return [
    {
      name: "Responses",
      ok: totals.ok,
      redirect: totals.redirect,
      clientError: totals.clientError,
      serverError: totals.serverError,
    },
  ];
}

export function buildRoutePerformance(
  metrics: ApiMetricsSnapshot | null,
): Array<{ route: string; requests: number; avgLatencyMs: number }> {
  if (!metrics) {
    return [];
  }

  return [...metrics.routes]
    .sort((a, b) => b.requests - a.requests)
    .slice(0, 8)
    .map((route) => ({
      route: compactRoute(route.route),
      requests: route.requests,
      avgLatencyMs: Number(route.avgLatencyMs.toFixed(2)),
    }));
}

export function buildSlowRoutes(
  metrics: ApiMetricsSnapshot | null,
): Array<{ route: string; avgLatencyMs: number; normalized: number }> {
  if (!metrics || metrics.routes.length === 0) {
    return [];
  }

  const rows = [...metrics.routes]
    .sort((a, b) => b.avgLatencyMs - a.avgLatencyMs)
    .slice(0, 6)
    .map((route) => ({ route: compactRoute(route.route), avgLatencyMs: route.avgLatencyMs }));
  const peak = Math.max(...rows.map((row) => row.avgLatencyMs), 1);

  return rows.map((row) => ({
    ...row,
    normalized: Math.max(12, (row.avgLatencyMs / peak) * 100),
  }));
}

export function compactRoute(route: string): string {
  if (route.length <= 26) {
    return route;
  }
  return `${route.slice(0, 23)}...`;
}

export function parsePercentValue(value: string): number {
  const num = Number.parseFloat(value.replace("%", ""));
  if (!Number.isFinite(num)) {
    return 0;
  }
  return Math.min(Math.max(num, 0), 100);
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

export function parseCPUMilli(value: string): number {
  const normalized = value.trim().toLowerCase();
  if (!normalized || normalized === "n/a") {
    return 0;
  }

  if (normalized.endsWith("m")) {
    const parsed = Number.parseFloat(normalized.slice(0, -1));
    return Number.isFinite(parsed) ? parsed : 0;
  }

  const parsed = Number.parseFloat(normalized);
  if (!Number.isFinite(parsed)) {
    return 0;
  }

  return parsed * 1000;
}

export function parseMemoryMi(value: string): number {
  const normalized = value.trim().toLowerCase();
  if (!normalized || normalized === "n/a") {
    return 0;
  }

  if (normalized.endsWith("mi")) {
    const parsed = Number.parseFloat(normalized.slice(0, -2));
    return Number.isFinite(parsed) ? parsed : 0;
  }

  if (normalized.endsWith("gi")) {
    const parsed = Number.parseFloat(normalized.slice(0, -2));
    return Number.isFinite(parsed) ? parsed * 1024 : 0;
  }

  if (normalized.endsWith("ki")) {
    const parsed = Number.parseFloat(normalized.slice(0, -2));
    return Number.isFinite(parsed) ? parsed / 1024 : 0;
  }

  const parsed = Number.parseFloat(normalized.replace(/b$/, ""));
  if (!Number.isFinite(parsed)) {
    return 0;
  }

  return parsed / (1024 * 1024);
}

export function percentage(part: number, whole: number): number {
  if (!Number.isFinite(part) || !Number.isFinite(whole) || whole <= 0) {
    return 0;
  }
  return Number(((part / whole) * 100).toFixed(2));
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
