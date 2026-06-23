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
import { ACCENT, AMBER, AXIS_TICK, BLUE, GRID_BASELINE, RED, TOOLTIP_STYLE } from "../constants";
import { coerceNumber } from "../utils";
import { ChartCard, EmptyChart, StackBar } from "./DashboardPrimitives";
import { PodLifecycleMix } from "./PodLifecycleMix";

interface DashboardChartsProps {
  podMixData: Array<{ name: string; value: number; color: string }>;
  nodeUsageBars: Array<{ name: string; cpu: number; memory: number }>;
  nodeCPUTrend: Array<{ time: string; value: number }>;
  eventReasonBars: Array<{ name: string; count: number }>;
  restartHotspots: Array<{ name: string; restarts: number; color: string }>;
  apiStatusMix: { ok: number; redirect: number; clientError: number; serverError: number; total: number };
  onOpenPods: () => void;
  onOpenNodes: () => void;
  onOpenEvents: () => void;
  onOpenAudit: () => void;
}

export function DashboardCharts({
  podMixData,
  nodeUsageBars,
  nodeCPUTrend,
  eventReasonBars,
  restartHotspots,
  apiStatusMix,
  onOpenPods,
  onOpenNodes,
  onOpenEvents,
  onOpenAudit,
}: DashboardChartsProps) {
  return (
    <section className="grid grid-cols-1 xl:grid-cols-2 gap-4">
      <ChartCard
        title="Pod Lifecycle Mix"
        subtitle="Composition of running, pending, failed, and succeeded pods."
        onDrilldown={onOpenPods}
        drilldownLabel="Open Pods"
      >
        <PodLifecycleMix data={podMixData} />
      </ChartCard>

      <NodeUtilizationChart nodeUsageBars={nodeUsageBars} onOpenNodes={onOpenNodes} />
      <NodeCPUTrendChart nodeCPUTrend={nodeCPUTrend} onOpenNodes={onOpenNodes} />
      <EventReasonChart eventReasonBars={eventReasonBars} onOpenEvents={onOpenEvents} />
      <RestartHotspotsChart restartHotspots={restartHotspots} onOpenPods={onOpenPods} />
      <APIStatusMixCard apiStatusMix={apiStatusMix} onOpenAudit={onOpenAudit} />
    </section>
  );
}

function NodeUtilizationChart({
  nodeUsageBars,
  onOpenNodes,
}: {
  nodeUsageBars: DashboardChartsProps["nodeUsageBars"];
  onOpenNodes: () => void;
}) {
  return (
    <ChartCard
      title="Node Utilization"
      subtitle="CPU and memory percentage by node (top 8)."
      onDrilldown={onOpenNodes}
      drilldownLabel="Open Nodes"
    >
      {nodeUsageBars.length > 0 ? (
        <ResponsiveContainer width="100%" height={320}>
          <BarChart data={nodeUsageBars} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={onOpenNodes}>
            <ReferenceLine y={0} stroke={GRID_BASELINE} />
            <XAxis dataKey="name" tick={AXIS_TICK} interval={0} angle={-20} textAnchor="end" height={48} />
            <YAxis domain={[0, 100]} tick={AXIS_TICK} unit="%" />
            <Tooltip
              contentStyle={TOOLTIP_STYLE}
              formatter={(value, key) => {
                const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                const label = key === "cpu" ? "CPU" : "Memory";
                return [`${numeric.toFixed(1)}%`, label] as [string, string];
              }}
            />
            <Bar dataKey="cpu" fill={ACCENT} onClick={onOpenNodes} cursor="pointer" />
            <Bar dataKey="memory" fill="rgba(0, 212, 168, 0.4)" onClick={onOpenNodes} cursor="pointer" />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <EmptyChart message="No node utilization data available." />
      )}
    </ChartCard>
  );
}

function NodeCPUTrendChart({
  nodeCPUTrend,
  onOpenNodes,
}: {
  nodeCPUTrend: DashboardChartsProps["nodeCPUTrend"];
  onOpenNodes: () => void;
}) {
  return (
    <ChartCard
      title="Average Node CPU Trend"
      subtitle="Cluster-wide CPU trajectory from node history points."
      onDrilldown={onOpenNodes}
      drilldownLabel="Open Nodes"
    >
      {nodeCPUTrend.length > 0 ? (
        <ResponsiveContainer width="100%" height={320}>
          <LineChart data={nodeCPUTrend} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={onOpenNodes}>
            <XAxis dataKey="time" tick={AXIS_TICK} />
            <YAxis domain={[0, 100]} tick={AXIS_TICK} unit="%" />
            <Tooltip
              contentStyle={TOOLTIP_STYLE}
              formatter={(value) => {
                const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                return [`${numeric.toFixed(1)}%`, "Avg CPU"] as [string, string];
              }}
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
  );
}

function EventReasonChart({
  eventReasonBars,
  onOpenEvents,
}: {
  eventReasonBars: DashboardChartsProps["eventReasonBars"];
  onOpenEvents: () => void;
}) {
  return (
    <ChartCard
      title="Event Reason Frequency"
      subtitle="Most common event reasons across recent cluster events."
      onDrilldown={onOpenEvents}
      drilldownLabel="Open Events"
    >
      {eventReasonBars.length > 0 ? (
        <ResponsiveContainer width="100%" height={320}>
          <BarChart data={eventReasonBars} margin={{ top: 6, right: 8, left: 0, bottom: 0 }} onClick={onOpenEvents}>
            <ReferenceLine y={0} stroke={GRID_BASELINE} />
            <XAxis dataKey="name" tick={AXIS_TICK} interval={0} angle={-20} textAnchor="end" height={48} />
            <YAxis allowDecimals={false} tick={AXIS_TICK} />
            <Tooltip
              contentStyle={TOOLTIP_STYLE}
              formatter={(value) => {
                const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                return [Math.round(numeric), "Events"] as [number, string];
              }}
            />
            <Bar dataKey="count" fill={BLUE} onClick={onOpenEvents} cursor="pointer" />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <EmptyChart message="No event reason data available." />
      )}
    </ChartCard>
  );
}

function RestartHotspotsChart({
  restartHotspots,
  onOpenPods,
}: {
  restartHotspots: DashboardChartsProps["restartHotspots"];
  onOpenPods: () => void;
}) {
  return (
    <ChartCard
      title="Restart Hotspots"
      subtitle="Pods with highest restart pressure."
      onDrilldown={onOpenPods}
      drilldownLabel="Open Pods"
    >
      {restartHotspots.length > 0 ? (
        <ResponsiveContainer width="100%" height={320}>
          <BarChart
            data={restartHotspots}
            layout="vertical"
            margin={{ top: 6, right: 8, left: 16, bottom: 0 }}
            onClick={onOpenPods}
          >
            <ReferenceLine x={0} stroke={GRID_BASELINE} />
            <XAxis type="number" allowDecimals={false} tick={AXIS_TICK} />
            <YAxis type="category" dataKey="name" width={130} tick={AXIS_TICK} />
            <Tooltip
              contentStyle={TOOLTIP_STYLE}
              formatter={(value) => {
                const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                return [Math.round(numeric), "Restarts"] as [number, string];
              }}
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
  );
}

function APIStatusMixCard({
  apiStatusMix,
  onOpenAudit,
}: {
  apiStatusMix: DashboardChartsProps["apiStatusMix"];
  onOpenAudit: () => void;
}) {
  return (
    <ChartCard
      title="API Response Class Mix"
      subtitle="2xx, 3xx, 4xx, and 5xx totals from API observability."
      onDrilldown={onOpenAudit}
      drilldownLabel="Open Audit"
    >
      {apiStatusMix.total > 0 ? (
        <div className="space-y-3 pt-1">
          <button type="button" onClick={onOpenAudit} className="w-full text-left">
            <StackBar label="2xx" value={apiStatusMix.ok} total={apiStatusMix.total} color="#e8e8e8" />
          </button>
          <button type="button" onClick={onOpenAudit} className="w-full text-left">
            <StackBar label="3xx" value={apiStatusMix.redirect} total={apiStatusMix.total} color="#666666" />
          </button>
          <button type="button" onClick={onOpenAudit} className="w-full text-left">
            <StackBar label="4xx" value={apiStatusMix.clientError} total={apiStatusMix.total} color={AMBER} />
          </button>
          <button type="button" onClick={onOpenAudit} className="w-full text-left">
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
  );
}
