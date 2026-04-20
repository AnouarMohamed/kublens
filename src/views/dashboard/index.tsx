import {
  Bar,
  BarChart,
  Cell,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { navigateToView } from "../../app/viewNavigation";
import { BLUE, AMBER, RED, ACCENT, AXIS_TICK, GRID_BASELINE, TOOLTIP_STYLE } from "./constants";
import { DashboardHeader } from "./components/DashboardHeader";
import { DashboardKpiBand } from "./components/DashboardKpiBand";
import { ChartCard, EmptyChart, RiskRail, StackBar } from "./components/DashboardPrimitives";
import { PodLifecycleMix } from "./components/PodLifecycleMix";
import { DiagnosticsNarrative } from "./components/DiagnosticsNarrative";
import { HealthSnapshotCard, RecentEventsCard, TopRiskPodsCard } from "./components/DashboardInsights";
import { useDashboardData } from "./hooks/useDashboardData";
import { coerceNumber } from "./utils";

export default function Dashboard() {
  const {
    stats,
    diagnostics,
    healthHistory,
    events,
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
    kpis,
  } = useDashboardData();
  const openPods = () => navigateToView("pods");
  const openNodes = () => navigateToView("nodes");
  const openEvents = () => navigateToView("events");
  const openAudit = () => navigateToView("audit");
  const openDiagnostics = () => navigateToView("diagnostics");

  return (
    <div className="space-y-6">
      <DashboardHeader stats={stats} isLoading={isLoading} onRefresh={() => void load()} />

      {error && (
        <div className="rounded-xl border border-zinc-700 bg-zinc-900/80 px-3 py-2 text-sm text-zinc-200">{error}</div>
      )}

      <DashboardKpiBand kpis={kpis} />

      <section className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <RiskRail label="Pending Pod Rate" value={pendingRate} onClick={openPods} />
        <RiskRail label="Failed Pod Rate" value={failedRate} onClick={openPods} />
        <RiskRail label="NotReady Node Rate" value={notReadyRate} onClick={openNodes} />
      </section>

      <section className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <ChartCard
          title="Pod Lifecycle Mix"
          subtitle="Composition of running, pending, failed, and succeeded pods."
          onDrilldown={openPods}
          drilldownLabel="Open Pods"
        >
          <PodLifecycleMix data={podMixData} />
        </ChartCard>

        <ChartCard
          title="Node Utilization"
          subtitle="CPU and memory percentage by node (top 8)."
          onDrilldown={openNodes}
          drilldownLabel="Open Nodes"
        >
          {nodeUsageBars.length > 0 ? (
            <ResponsiveContainer width="100%" height={320}>
              <BarChart data={nodeUsageBars} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={openNodes}>
                <ReferenceLine y={0} stroke={GRID_BASELINE} />
                <XAxis dataKey="name" tick={AXIS_TICK} interval={0} angle={-20} textAnchor="end" height={48} />
                <YAxis domain={[0, 100]} tick={AXIS_TICK} unit="%" />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: number | string | undefined, key: string | undefined) => [
                    `${coerceNumber(value).toFixed(1)}%`,
                    key === "cpu" ? "CPU" : "Memory",
                  ]}
                />
                <Bar dataKey="cpu" fill={ACCENT} onClick={openNodes} cursor="pointer" />
                <Bar dataKey="memory" fill="rgba(0, 212, 168, 0.4)" onClick={openNodes} cursor="pointer" />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart message="No node utilization data available." />
          )}
        </ChartCard>

        <ChartCard
          title="Average Node CPU Trend"
          subtitle="Cluster-wide CPU trajectory from node history points."
          onDrilldown={openNodes}
          drilldownLabel="Open Nodes"
        >
          {nodeCPUTrend.length > 0 ? (
            <ResponsiveContainer width="100%" height={320}>
              <LineChart data={nodeCPUTrend} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={openNodes}>
                <XAxis dataKey="time" tick={AXIS_TICK} />
                <YAxis domain={[0, 100]} tick={AXIS_TICK} unit="%" />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: number | string | undefined) => [`${coerceNumber(value).toFixed(1)}%`, "Avg CPU"]}
                />
                <Line
                  type="monotone"
                  dataKey="value"
                  stroke={ACCENT}
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={{ r: 3, fill: ACCENT, stroke: ACCENT }}
                  cursor="pointer"
                />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart message="No CPU history points available." />
          )}
        </ChartCard>

        <ChartCard
          title="Event Reason Frequency"
          subtitle="Most common event reasons across recent cluster events."
          onDrilldown={openEvents}
          drilldownLabel="Open Events"
        >
          {eventReasonBars.length > 0 ? (
            <ResponsiveContainer width="100%" height={320}>
              <BarChart data={eventReasonBars} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={openEvents}>
                <ReferenceLine y={0} stroke={GRID_BASELINE} />
                <XAxis dataKey="name" tick={AXIS_TICK} interval={0} angle={-20} textAnchor="end" height={48} />
                <YAxis allowDecimals={false} tick={AXIS_TICK} />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: number | string | undefined) => [Math.round(coerceNumber(value)), "Events"]}
                />
                <Bar dataKey="count" fill={BLUE} onClick={openEvents} cursor="pointer" />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart message="No event reason data available." />
          )}
        </ChartCard>

        <ChartCard
          title="Restart Hotspots"
          subtitle="Pods with highest restart pressure."
          onDrilldown={openPods}
          drilldownLabel="Open Pods"
        >
          {restartHotspots.length > 0 ? (
            <ResponsiveContainer width="100%" height={320}>
              <BarChart
                data={restartHotspots}
                layout="vertical"
                margin={{ top: 6, right: 8, left: 16, bottom: 0 }}
                onClick={openPods}
              >
                <ReferenceLine x={0} stroke={GRID_BASELINE} />
                <XAxis type="number" allowDecimals={false} tick={AXIS_TICK} />
                <YAxis type="category" dataKey="name" width={130} tick={AXIS_TICK} />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: number | string | undefined) => [Math.round(coerceNumber(value)), "Restarts"]}
                />
                <Bar dataKey="restarts" cursor="pointer">
                  {restartHotspots.map((row) => (
                    <Cell key={row.name} fill={row.color} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <EmptyChart message="No restart hotspot data available." />
          )}
        </ChartCard>

        <ChartCard
          title="API Response Class Mix"
          subtitle="2xx, 3xx, 4xx, and 5xx totals from API observability."
          onDrilldown={openAudit}
          drilldownLabel="Open Audit"
        >
          {apiStatusMix.total > 0 ? (
            <div className="space-y-3 pt-1">
              <button type="button" onClick={openAudit} className="w-full text-left">
                <StackBar label="2xx" value={apiStatusMix.ok} total={apiStatusMix.total} color={ACCENT} />
              </button>
              <button type="button" onClick={openAudit} className="w-full text-left">
                <StackBar label="3xx" value={apiStatusMix.redirect} total={apiStatusMix.total} color={BLUE} />
              </button>
              <button type="button" onClick={openAudit} className="w-full text-left">
                <StackBar label="4xx" value={apiStatusMix.clientError} total={apiStatusMix.total} color={AMBER} />
              </button>
              <button type="button" onClick={openAudit} className="w-full text-left">
                <StackBar label="5xx" value={apiStatusMix.serverError} total={apiStatusMix.total} color={RED} />
              </button>
              <div className="rounded-lg border border-zinc-700 bg-zinc-900/60 px-3 py-2 text-xs text-zinc-300">
                Total responses observed: <span className="font-semibold text-zinc-100">{apiStatusMix.total}</span>
              </div>
            </div>
          ) : (
            <EmptyChart message="No API status metrics available." />
          )}
        </ChartCard>
      </section>

      <section className="grid grid-cols-1 xl:grid-cols-3 gap-4">
        <TopRiskPodsCard pods={topRiskPods} />
        <RecentEventsCard events={events} />
        <HealthSnapshotCard diagnostics={diagnostics} healthHistory={healthHistory} />
      </section>

      <div className="flex justify-end">
        <button type="button" onClick={openDiagnostics} className="btn-sm">
          Open Diagnostics
        </button>
      </div>
      <DiagnosticsNarrative diagnostics={diagnostics} stats={stats} prioritizedIssues={prioritizedIssues} />
    </div>
  );
}
