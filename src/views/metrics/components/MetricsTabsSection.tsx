import type { AnalyticsTab } from "../hooks/useMetricsData";
import { TabButton } from "./MetricsPrimitives";
import type { RAGTelemetry } from "../../../types";
import { APITabPanel, AssistantQualityTabPanel, ClusterTabPanel, WorkloadsTabPanel } from "./MetricsTabPanels";

interface MetricsTabsSectionProps {
  tab: AnalyticsTab;
  onTabChange: (tab: AnalyticsTab) => void;
  nodeUtilizationBars: Array<{ name: string; cpu: number; memory: number }>;
  nodeCPUTrend: Array<{ time: string; value: number }>;
  podLifecycleBars: Array<{ name: string; value: number; color: string }>;
  restartBands: Array<{ name: string; value: number; color: string }>;
  podPressureBars: Array<{ name: string; score: number; cpuMilli: number; memMi: number; color: string }>;
  apiStatusStack: Array<{ name: string; ok: number; redirect: number; clientError: number; serverError: number }>;
  routePerformance: Array<{ route: string; requests: number; avgLatencyMs: number }>;
  ragTelemetry: RAGTelemetry | null;
  ragEmptyRate: number;
  ragFeedbackBalance: number;
}

export function MetricsTabsSection({
  tab,
  onTabChange,
  nodeUtilizationBars,
  nodeCPUTrend,
  podLifecycleBars,
  restartBands,
  podPressureBars,
  apiStatusStack,
  routePerformance,
  ragTelemetry,
  ragEmptyRate,
  ragFeedbackBalance,
}: MetricsTabsSectionProps) {
  return (
    <section className="surface p-4">
      <div className="flex flex-wrap items-center gap-2">
        <TabButton label="Cluster" active={tab === "cluster"} onClick={() => onTabChange("cluster")} />
        <TabButton label="Workloads" active={tab === "workloads"} onClick={() => onTabChange("workloads")} />
        <TabButton label="API" active={tab === "api"} onClick={() => onTabChange("api")} />
        <TabButton label="Assistant Quality" active={tab === "assistant"} onClick={() => onTabChange("assistant")} />
      </div>

      {tab === "cluster" && <ClusterTabPanel nodeUtilizationBars={nodeUtilizationBars} nodeCPUTrend={nodeCPUTrend} />}
      {tab === "workloads" && (
        <WorkloadsTabPanel
          podLifecycleBars={podLifecycleBars}
          restartBands={restartBands}
          podPressureBars={podPressureBars}
        />
      )}
      {tab === "api" && <APITabPanel apiStatusStack={apiStatusStack} routePerformance={routePerformance} />}
      {tab === "assistant" && (
        <AssistantQualityTabPanel
          ragTelemetry={ragTelemetry}
          ragEmptyRate={ragEmptyRate}
          ragFeedbackBalance={ragFeedbackBalance}
        />
      )}
    </section>
  );
}
