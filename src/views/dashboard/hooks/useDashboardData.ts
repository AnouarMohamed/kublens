import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toErrorMessage } from "../../../app/hooks/asyncTask";
import { api } from "../../../lib/api";
import type { ApiMetricsSnapshot, ClusterStats, DiagnosticsResult, K8sEvent, Node, Pod } from "../../../types";
import {
  buildAPIStatusMix,
  buildEventReasonBars,
  buildNodeCPUTrend,
  buildNodeUsageBars,
  buildPodMixData,
  buildPrioritizedIssues,
  buildRestartHotspots,
  percentage,
} from "../utils";

const DASHBOARD_REFRESH_MS = 30000;
const HEALTH_HISTORY_KEY = "kubelens:health-history";
const MAX_HEALTH_HISTORY = 20;

interface DashboardKpi {
  label: string;
  value: string;
  critical: boolean;
}

/**
 * Dashboard data dependencies and derived chart datasets.
 */
interface UseDashboardDataResult {
  stats: ClusterStats | null;
  diagnostics: DiagnosticsResult | null;
  healthHistory: Array<{ t: number; score: number }>;
  events: K8sEvent[];
  nodes: Node[];
  pods: Pod[];
  apiMetrics: ApiMetricsSnapshot | null;
  isLoading: boolean;
  error: string | null;
  load: () => Promise<void>;
  topRiskPods: Pod[];
  prioritizedIssues: DiagnosticsResult["issues"];
  podMixData: Array<{ name: string; value: number; color: string }>;
  nodeUsageBars: Array<{ name: string; cpu: number; memory: number }>;
  nodeCPUTrend: Array<{ time: string; value: number }>;
  restartHotspots: Array<{ name: string; restarts: number; color: string }>;
  eventReasonBars: Array<{ name: string; count: number }>;
  apiStatusMix: {
    ok: number;
    redirect: number;
    clientError: number;
    serverError: number;
    total: number;
  };
  pendingRate: number;
  failedRate: number;
  notReadyRate: number;
  nodeAvailabilityPercent: number;
  kpis: DashboardKpi[];
}

/**
 * Loads dashboard source data and computes all derived view models.
 *
 * @returns Dashboard state, loaders, and chart-friendly computed rows.
 */
export function useDashboardData(): UseDashboardDataResult {
  const [stats, setStats] = useState<ClusterStats | null>(null);
  const [diagnostics, setDiagnostics] = useState<DiagnosticsResult | null>(null);
  const [healthHistory, setHealthHistory] = useState<Array<{ t: number; score: number }>>(() => readHealthHistory());
  const [events, setEvents] = useState<K8sEvent[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [pods, setPods] = useState<Pod[]>([]);
  const [apiMetrics, setApiMetrics] = useState<ApiMetricsSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const requestSeqRef = useRef(0);
  const activeControllerRef = useRef<AbortController | null>(null);

  const loadWithMode = useCallback(async (backgroundRefresh: boolean) => {
    const requestID = requestSeqRef.current + 1;
    requestSeqRef.current = requestID;

    activeControllerRef.current?.abort();
    const controller = new AbortController();
    activeControllerRef.current = controller;

    if (!backgroundRefresh) {
      setIsLoading(true);
    }

    try {
      const [statsResponse, diagnosticsResponse, eventsResponse, nodesResponse, podsResponse, apiMetricsResponse] =
        await Promise.all([
          api.getStats(controller.signal),
          api.getDiagnostics(controller.signal),
          api.getEvents(controller.signal),
          api.getNodes(controller.signal),
          api.getPods(controller.signal),
          api.getApiMetrics(controller.signal),
        ]);

      if (controller.signal.aborted || requestID !== requestSeqRef.current) {
        return;
      }

      setStats(statsResponse);
      setDiagnostics(diagnosticsResponse);
      if (Number.isFinite(diagnosticsResponse.healthScore)) {
        setHealthHistory((current) =>
          appendHealthHistoryPoint(current, {
            t: Date.now(),
            score: diagnosticsResponse.healthScore,
          }),
        );
      }
      setEvents(eventsResponse);
      setNodes(nodesResponse);
      setPods(podsResponse);
      setApiMetrics(apiMetricsResponse);
      setError(null);
    } catch (err) {
      if (controller.signal.aborted || requestID !== requestSeqRef.current) {
        return;
      }
      setError(toErrorMessage(err, "Failed to load dashboard data"));
    } finally {
      if (activeControllerRef.current === controller) {
        activeControllerRef.current = null;
      }
      if (!backgroundRefresh && requestID === requestSeqRef.current) {
        setIsLoading(false);
      }
    }
  }, []);

  const load = useCallback(async () => {
    await loadWithMode(false);
  }, [loadWithMode]);

  useEffect(() => {
    void loadWithMode(false);

    const timerID = window.setInterval(() => {
      void loadWithMode(true);
    }, DASHBOARD_REFRESH_MS);

    return () => {
      window.clearInterval(timerID);
      requestSeqRef.current += 1;
      activeControllerRef.current?.abort();
      activeControllerRef.current = null;
    };
  }, [loadWithMode]);

  useEffect(() => {
    writeHealthHistory(healthHistory);
  }, [healthHistory]);

  const topRiskPods = useMemo(() => [...pods].sort((a, b) => b.restarts - a.restarts).slice(0, 6), [pods]);
  const prioritizedIssues = useMemo(() => buildPrioritizedIssues(diagnostics), [diagnostics]);
  const podMixData = useMemo(() => buildPodMixData(stats), [stats]);
  const nodeUsageBars = useMemo(() => buildNodeUsageBars(nodes), [nodes]);
  const nodeCPUTrend = useMemo(() => buildNodeCPUTrend(nodes), [nodes]);
  const restartHotspots = useMemo(() => buildRestartHotspots(pods), [pods]);
  const eventReasonBars = useMemo(() => buildEventReasonBars(events), [events]);
  const apiStatusMix = useMemo(() => buildAPIStatusMix(apiMetrics), [apiMetrics]);

  const pendingRate = useMemo(() => percentage(stats?.pods.pending ?? 0, stats?.pods.total ?? 0), [stats]);
  const failedRate = useMemo(() => percentage(stats?.pods.failed ?? 0, stats?.pods.total ?? 0), [stats]);
  const notReadyRate = useMemo(() => percentage(stats?.nodes.notReady ?? 0, stats?.nodes.total ?? 0), [stats]);
  const nodeAvailabilityPercent = useMemo(() => percentage(stats?.nodes.ready ?? 0, stats?.nodes.total ?? 0), [stats]);

  const kpis = useMemo(
    () => [
      { label: "Cluster CPU", value: stats?.cluster.cpu ?? "-", critical: false },
      { label: "Cluster Memory", value: stats?.cluster.memory ?? "-", critical: false },
      { label: "Failed Rate", value: `${failedRate.toFixed(1)}%`, critical: failedRate > 0 },
      {
        label: "Node Availability",
        value: `${stats?.nodes.ready ?? 0}/${stats?.nodes.total ?? 0}`,
        critical: nodeAvailabilityPercent > 0 && nodeAvailabilityPercent < 80,
      },
    ],
    [stats, failedRate, nodeAvailabilityPercent],
  );

  return {
    stats,
    diagnostics,
    healthHistory,
    events,
    nodes,
    pods,
    apiMetrics,
    isLoading,
    error,
    load,
    topRiskPods,
    prioritizedIssues,
    podMixData,
    nodeUsageBars,
    nodeCPUTrend,
    restartHotspots,
    eventReasonBars,
    apiStatusMix,
    pendingRate,
    failedRate,
    notReadyRate,
    nodeAvailabilityPercent,
    kpis,
  };
}

function readHealthHistory(): Array<{ t: number; score: number }> {
  if (typeof window === "undefined") {
    return [];
  }

  try {
    const raw = window.sessionStorage.getItem(HEALTH_HISTORY_KEY);
    if (!raw) {
      return [];
    }

    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }

    return parsed.filter(isHealthHistoryPoint).slice(-MAX_HEALTH_HISTORY);
  } catch {
    return [];
  }
}

function writeHealthHistory(history: Array<{ t: number; score: number }>): void {
  if (typeof window === "undefined") {
    return;
  }

  try {
    window.sessionStorage.setItem(HEALTH_HISTORY_KEY, JSON.stringify(history.slice(-MAX_HEALTH_HISTORY)));
  } catch {
    // Session storage is best effort for client-side chart history.
  }
}

function appendHealthHistoryPoint(
  history: Array<{ t: number; score: number }>,
  point: { t: number; score: number },
): Array<{ t: number; score: number }> {
  return [...history, point].slice(-MAX_HEALTH_HISTORY);
}

function isHealthHistoryPoint(value: unknown): value is { t: number; score: number } {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as { t?: unknown; score?: unknown };
  return Number.isFinite(candidate.t) && Number.isFinite(candidate.score);
}
