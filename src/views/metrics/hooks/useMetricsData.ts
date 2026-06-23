import { useCallback, useMemo, useState } from "react";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { api } from "../../../lib/api";
import type { ApiMetricsSnapshot, ClusterStats, Node, Pod, RAGTelemetry } from "../../../types";
import {
  buildAPIStatusStack,
  buildAPIStatusTotals,
  buildNodeCPUTrend,
  buildNodeUtilizationBars,
  buildPodLifecycleBars,
  buildRestartBands,
  buildRoutePerformance,
  buildSlowRoutes,
  buildTopPodPressure,
  percentage,
} from "../utils";

export type AnalyticsTab = "cluster" | "workloads" | "api" | "assistant";

interface MetricsKpi {
  label: string;
  value: string;
}

interface MetricsPayload {
  stats: ClusterStats | null;
  nodes: Node[];
  pods: Pod[];
  apiMetrics: ApiMetricsSnapshot | null;
  ragTelemetry: RAGTelemetry | null;
}

const METRICS_REFRESH_MS = 15000;
const EMPTY_METRICS_PAYLOAD: MetricsPayload = {
  stats: null,
  nodes: [],
  pods: [],
  apiMetrics: null,
  ragTelemetry: null,
};

/**
 * State and derived datasets for the metrics view.
 */
interface UseMetricsDataResult {
  stats: ClusterStats | null;
  nodes: Node[];
  pods: Pod[];
  apiMetrics: ApiMetricsSnapshot | null;
  ragTelemetry: RAGTelemetry | null;
  isLoading: boolean;
  autoRefresh: boolean;
  tab: AnalyticsTab;
  error: string | null;
  setAutoRefresh: (value: boolean) => void;
  setTab: (tab: AnalyticsTab) => void;
  load: () => Promise<void>;
  requestRatePerMinute: number;
  apiStatusTotals: { ok: number; redirect: number; clientError: number; serverError: number; total: number };
  errorRate: number;
  ragEmptyRate: number;
  ragFeedbackBalance: number;
  nodeReadiness: number;
  podStability: number;
  apiSuccess: number;
  podLifecycleBars: Array<{ name: string; value: number; color: string }>;
  nodeUtilizationBars: Array<{ name: string; cpu: number; memory: number }>;
  nodeCPUTrend: Array<{ time: string; value: number }>;
  restartBands: Array<{ name: string; value: number; color: string }>;
  podPressureBars: Array<{ name: string; score: number; cpuMilli: number; memMi: number; color: string }>;
  apiStatusStack: Array<{ name: string; ok: number; redirect: number; clientError: number; serverError: number }>;
  routePerformance: Array<{ route: string; requests: number; avgLatencyMs: number }>;
  slowRoutes: Array<{ route: string; avgLatencyMs: number; normalized: number }>;
  kpiItems: MetricsKpi[];
}

/**
 * Loads metrics dependencies and computes chart-ready datasets.
 *
 * @returns Metrics state, controls, and computed data rows.
 */
export function useMetricsData(): UseMetricsDataResult {
  const [autoRefresh, setAutoRefreshState] = useState(true);
  const [tab, setTabState] = useState<AnalyticsTab>("cluster");

  const setAutoRefresh = useCallback((value: boolean) => {
    setAutoRefreshState(value);
  }, []);

  const setTab = useCallback((nextTab: AnalyticsTab) => {
    setTabState(nextTab);
  }, []);

  const loadMetricsPayload = useCallback(async (signal: AbortSignal): Promise<MetricsPayload> => {
    const [statsPayload, nodesPayload, podsPayload, metricsPayload, ragPayload] = await Promise.all([
      api.getStats(signal),
      api.getNodes(signal),
      api.getPods(signal),
      api.getApiMetrics(signal),
      api.getRAGTelemetry(24, signal).catch(() => null),
    ]);

    return {
      stats: statsPayload,
      nodes: nodesPayload,
      pods: podsPayload,
      apiMetrics: metricsPayload,
      ragTelemetry: ragPayload ?? {
        enabled: metricsPayload.rag.enabled,
        indexedAt: "",
        expiresAt: "",
        totalQueries: metricsPayload.rag.totalQueries,
        emptyResults: metricsPayload.rag.emptyResults,
        hitRate: metricsPayload.rag.hitRate,
        averageResults: metricsPayload.rag.averageResults,
        feedbackSignals: metricsPayload.rag.feedbackSignals,
        positiveFeedback: metricsPayload.rag.positiveFeedback,
        negativeFeedback: metricsPayload.rag.negativeFeedback,
        topFeedbackDocs: [],
        recentQueries: [],
      },
    };
  }, []);

  const {
    data: { stats, nodes, pods, apiMetrics, ragTelemetry },
    isLoading,
    error,
    load,
  } = useAsyncResource({
    loader: loadMetricsPayload,
    fallbackError: "Failed to load metrics",
    initialData: EMPTY_METRICS_PAYLOAD,
    refreshMs: METRICS_REFRESH_MS,
    refreshEnabled: autoRefresh,
  });

  const requestRatePerMinute = useMemo(() => {
    if (!apiMetrics || apiMetrics.uptimeSeconds <= 0) {
      return 0;
    }
    return (apiMetrics.totalRequests / apiMetrics.uptimeSeconds) * 60;
  }, [apiMetrics]);

  const apiStatusTotals = useMemo(() => buildAPIStatusTotals(apiMetrics), [apiMetrics]);

  const errorRate = useMemo(() => {
    if (!apiMetrics || apiMetrics.totalRequests === 0) {
      return 0;
    }
    return (apiMetrics.totalErrors / apiMetrics.totalRequests) * 100;
  }, [apiMetrics]);

  const ragEmptyRate = useMemo(() => {
    const totalQueries = ragTelemetry?.totalQueries ?? apiMetrics?.rag.totalQueries ?? 0;
    if (totalQueries === 0) {
      return 0;
    }
    return percentage(ragTelemetry?.emptyResults ?? apiMetrics?.rag.emptyResults ?? 0, totalQueries);
  }, [apiMetrics, ragTelemetry]);

  const ragFeedbackBalance = useMemo(() => {
    const positive = ragTelemetry?.positiveFeedback ?? apiMetrics?.rag.positiveFeedback ?? 0;
    const negative = ragTelemetry?.negativeFeedback ?? apiMetrics?.rag.negativeFeedback ?? 0;
    const total = positive + negative;
    if (total === 0) {
      return 0;
    }
    return percentage(positive, total);
  }, [apiMetrics, ragTelemetry]);

  const nodeReadiness = useMemo(() => {
    if (!stats) {
      return 0;
    }
    return percentage(stats.nodes.ready, stats.nodes.total);
  }, [stats]);

  const podStability = useMemo(() => {
    if (!stats) {
      return 0;
    }
    const succeeded = Math.max(stats.pods.total - stats.pods.running - stats.pods.pending - stats.pods.failed, 0);
    return percentage(stats.pods.running + succeeded, stats.pods.total);
  }, [stats]);

  const apiSuccess = useMemo(
    () => percentage(apiStatusTotals.ok + apiStatusTotals.redirect, apiStatusTotals.total),
    [apiStatusTotals],
  );

  const podLifecycleBars = useMemo(() => buildPodLifecycleBars(stats), [stats]);
  const nodeUtilizationBars = useMemo(() => buildNodeUtilizationBars(nodes), [nodes]);
  const nodeCPUTrend = useMemo(() => buildNodeCPUTrend(nodes), [nodes]);
  const restartBands = useMemo(() => buildRestartBands(pods), [pods]);
  const podPressureBars = useMemo(() => buildTopPodPressure(pods), [pods]);
  const apiStatusStack = useMemo(() => buildAPIStatusStack(apiStatusTotals), [apiStatusTotals]);
  const routePerformance = useMemo(() => buildRoutePerformance(apiMetrics), [apiMetrics]);
  const slowRoutes = useMemo(() => buildSlowRoutes(apiMetrics), [apiMetrics]);

  const kpiItems = useMemo(
    () => [
      { label: "Cluster CPU", value: stats?.cluster.cpu ?? "-" },
      { label: "Cluster Memory", value: stats?.cluster.memory ?? "-" },
      { label: "Pods", value: String(stats?.pods.total ?? 0) },
      { label: "Req/min", value: requestRatePerMinute.toFixed(1) },
      { label: "Avg Latency", value: `${(apiMetrics?.avgLatencyMs ?? 0).toFixed(1)}ms` },
      { label: "Error Rate", value: `${errorRate.toFixed(2)}%` },
      {
        label: "RAG Hit Rate",
        value: `${((ragTelemetry?.hitRate ?? apiMetrics?.rag.hitRate ?? 0) * 100).toFixed(1)}%`,
      },
    ],
    [stats, requestRatePerMinute, apiMetrics, errorRate, ragTelemetry],
  );

  return {
    stats,
    nodes,
    pods,
    apiMetrics,
    ragTelemetry,
    isLoading,
    autoRefresh,
    tab,
    error,
    setAutoRefresh,
    setTab,
    load,
    requestRatePerMinute,
    apiStatusTotals,
    errorRate,
    ragEmptyRate,
    ragFeedbackBalance,
    nodeReadiness,
    podStability,
    apiSuccess,
    podLifecycleBars,
    nodeUtilizationBars,
    nodeCPUTrend,
    restartBands,
    podPressureBars,
    apiStatusStack,
    routePerformance,
    slowRoutes,
    kpiItems,
  };
}
