import { MetricsHeader } from "./components/MetricsHeader";
import { MetricsKpiStrip } from "./components/MetricsKpiStrip";
import { SignalCard } from "./components/MetricsPrimitives";
import { MetricsRouteSections } from "./components/MetricsRouteSections";
import { MetricsTabsSection } from "./components/MetricsTabsSection";
import { useMetricsData } from "./hooks/useMetricsData";

export default function Metrics() {
  const {
    stats,
    apiMetrics,
    isLoading,
    autoRefresh,
    tab,
    error,
    ragTelemetry,
    setAutoRefresh,
    setTab,
    load,
    apiStatusTotals,
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
  } = useMetricsData();

  return (
    <div className="space-y-6">
      <MetricsHeader
        autoRefresh={autoRefresh}
        isLoading={isLoading}
        onAutoRefreshChange={setAutoRefresh}
        onRefresh={() => void load()}
      />

      {error && (
        <div className="rounded-xl border border-zinc-700 bg-zinc-900/85 px-3 py-2 text-sm text-zinc-200">{error}</div>
      )}

      <section className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <SignalCard
          label="Node Readiness"
          value={`${nodeReadiness.toFixed(1)}%`}
          detail={`${stats?.nodes.ready ?? 0}/${stats?.nodes.total ?? 0} nodes ready`}
          fill={nodeReadiness}
        />
        <SignalCard
          label="Pod Stability"
          value={`${podStability.toFixed(1)}%`}
          detail={`${stats?.pods.failed ?? 0} failed pods`}
          fill={podStability}
        />
        <SignalCard
          label="API Success"
          value={`${apiSuccess.toFixed(1)}%`}
          detail={`${apiStatusTotals.serverError} server errors`}
          fill={apiSuccess}
        />
      </section>

      <MetricsKpiStrip items={kpiItems} />

      <MetricsTabsSection
        tab={tab}
        onTabChange={setTab}
        nodeUtilizationBars={nodeUtilizationBars}
        nodeCPUTrend={nodeCPUTrend}
        podLifecycleBars={podLifecycleBars}
        restartBands={restartBands}
        podPressureBars={podPressureBars}
        apiStatusStack={apiStatusStack}
        routePerformance={routePerformance}
        ragTelemetry={ragTelemetry}
        ragEmptyRate={ragEmptyRate}
        ragFeedbackBalance={ragFeedbackBalance}
      />

      <MetricsRouteSections apiMetrics={apiMetrics} slowRoutes={slowRoutes} />
    </div>
  );
}
