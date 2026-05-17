import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  CHART_AMBER,
  CHART_BLUE,
  CHART_GREEN,
  CHART_MAGENTA,
  DOCKER_BLUE,
  GRID_STROKE,
  TOOLTIP_STYLE,
} from "../constants";
import { coerceNumber } from "../utils";
import { ChartCard, EmptyChart } from "./MetricsPrimitives";

interface ClusterTabPanelProps {
  nodeUtilizationBars: Array<{ name: string; cpu: number; memory: number }>;
  nodeCPUTrend: Array<{ time: string; value: number }>;
}

export function ClusterTabPanel({ nodeUtilizationBars, nodeCPUTrend }: ClusterTabPanelProps) {
  return (
    <div className="mt-4 grid grid-cols-1 xl:grid-cols-2 gap-4">
      <ChartCard title="Node Utilization by Node (CPU vs Memory)">
        {nodeUtilizationBars.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <BarChart data={nodeUtilizationBars} margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="name"
                tick={{ fill: "#5d6674", fontSize: 12 }}
                interval={0}
                angle={-20}
                textAnchor="end"
                height={52}
              />
              <YAxis domain={[0, 100]} tick={{ fill: "#5d6674", fontSize: 12 }} unit="%" />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                formatter={(value, name) => [`${coerceNumber(value).toFixed(1)}%`, name === "cpu" ? "CPU" : "Memory"]}
              />
              <Bar dataKey="cpu" name="CPU" fill={CHART_BLUE} radius={[4, 4, 0, 0]} />
              <Bar dataKey="memory" name="Memory" fill={CHART_GREEN} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No node utilization data available." />
        )}
      </ChartCard>

      <ChartCard title="Average Node CPU Trend">
        {nodeCPUTrend.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <AreaChart data={nodeCPUTrend} margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="time" tick={{ fill: "#5d6674", fontSize: 12 }} />
              <YAxis domain={[0, 100]} tick={{ fill: "#5d6674", fontSize: 12 }} unit="%" />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                formatter={(value) => [`${coerceNumber(value).toFixed(1)}%`, "Avg CPU"]}
              />
              <Area
                type="monotone"
                dataKey="value"
                stroke={DOCKER_BLUE}
                fill={CHART_BLUE}
                fillOpacity={0.22}
                strokeWidth={2}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No node CPU trend data available." />
        )}
      </ChartCard>
    </div>
  );
}

interface WorkloadsTabPanelProps {
  podLifecycleBars: Array<{ name: string; value: number; color: string }>;
  restartBands: Array<{ name: string; value: number; color: string }>;
  podPressureBars: Array<{ name: string; score: number; cpuMilli: number; memMi: number; color: string }>;
}

export function WorkloadsTabPanel({ podLifecycleBars, restartBands, podPressureBars }: WorkloadsTabPanelProps) {
  return (
    <div className="mt-4 grid grid-cols-1 xl:grid-cols-3 gap-4">
      <ChartCard title="Pod Lifecycle Distribution">
        {podLifecycleBars.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <BarChart data={podLifecycleBars} margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="name" tick={{ fill: "#5d6674", fontSize: 12 }} />
              <YAxis allowDecimals={false} tick={{ fill: "#5d6674", fontSize: 12 }} />
              <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(value) => [Math.round(coerceNumber(value)), "Pods"]} />
              <Bar dataKey="value" radius={[4, 4, 0, 0]}>
                {podLifecycleBars.map((row) => (
                  <Cell key={row.name} fill={row.color} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No pod lifecycle data available." />
        )}
      </ChartCard>

      <ChartCard title="Restart Pressure Bands">
        <ResponsiveContainer width="100%" height={340}>
          <BarChart data={restartBands} margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
            <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />
            <XAxis
              dataKey="name"
              tick={{ fill: "#5d6674", fontSize: 12 }}
              interval={0}
              angle={-18}
              textAnchor="end"
              height={50}
            />
            <YAxis allowDecimals={false} tick={{ fill: "#5d6674", fontSize: 12 }} />
            <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(value) => [Math.round(coerceNumber(value)), "Pods"]} />
            <Bar dataKey="value" radius={[4, 4, 0, 0]}>
              {restartBands.map((row) => (
                <Cell key={row.name} fill={row.color} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="Top Pod Resource Pressure">
        {podPressureBars.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <BarChart data={podPressureBars} layout="vertical" margin={{ top: 6, right: 10, left: 10, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" domain={[0, 100]} tick={{ fill: "#5d6674", fontSize: 12 }} unit="%" />
              <YAxis type="category" dataKey="name" width={120} tick={{ fill: "#5d6674", fontSize: 12 }} />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                formatter={(value, name, item) => {
                  const numeric = coerceNumber(value);
                  const payload = (item.payload ?? {}) as { cpuMilli?: number; memMi?: number };
                  if (name === "score") {
                    return [
                      `${numeric.toFixed(1)}%`,
                      `Pressure (CPU ${payload.cpuMilli ?? 0}m | Mem ${payload.memMi ?? 0}Mi)`,
                    ];
                  }
                  return [numeric, name];
                }}
              />
              <Bar dataKey="score" fill={DOCKER_BLUE} radius={[0, 4, 4, 0]} />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No pod pressure data available." />
        )}
      </ChartCard>
    </div>
  );
}

interface APITabPanelProps {
  apiStatusStack: Array<{ name: string; ok: number; redirect: number; clientError: number; serverError: number }>;
  routePerformance: Array<{ route: string; requests: number; avgLatencyMs: number }>;
}

export function APITabPanel({ apiStatusStack, routePerformance }: APITabPanelProps) {
  return (
    <div className="mt-4 grid grid-cols-1 xl:grid-cols-2 gap-4">
      <ChartCard title="HTTP Status Composition (Stacked)">
        {apiStatusStack.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <BarChart data={apiStatusStack} layout="vertical" margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" horizontal={false} />
              <XAxis type="number" tick={{ fill: "#5d6674", fontSize: 12 }} allowDecimals={false} />
              <YAxis type="category" dataKey="name" tick={{ fill: "#5d6674", fontSize: 12 }} width={84} />
              <Tooltip contentStyle={TOOLTIP_STYLE} />
              <Bar dataKey="ok" stackId="status" fill={CHART_GREEN} name="2xx" />
              <Bar dataKey="redirect" stackId="status" fill={CHART_BLUE} name="3xx" />
              <Bar dataKey="clientError" stackId="status" fill={CHART_AMBER} name="4xx" />
              <Bar dataKey="serverError" stackId="status" fill={CHART_MAGENTA} name="5xx" />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No HTTP status data available." />
        )}
      </ChartCard>

      <ChartCard title="Route Requests vs Latency">
        {routePerformance.length > 0 ? (
          <ResponsiveContainer width="100%" height={340}>
            <ComposedChart data={routePerformance} margin={{ top: 6, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={GRID_STROKE} strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="route"
                tick={{ fill: "#5d6674", fontSize: 12 }}
                interval={0}
                angle={-20}
                textAnchor="end"
                height={52}
              />
              <YAxis yAxisId="requests" tick={{ fill: "#5d6674", fontSize: 12 }} allowDecimals={false} />
              <YAxis yAxisId="latency" orientation="right" tick={{ fill: "#5d6674", fontSize: 12 }} />
              <Tooltip contentStyle={TOOLTIP_STYLE} />
              <Bar yAxisId="requests" dataKey="requests" fill={CHART_BLUE} radius={[4, 4, 0, 0]} name="Requests" />
              <Line
                yAxisId="latency"
                type="monotone"
                dataKey="avgLatencyMs"
                stroke={CHART_AMBER}
                strokeWidth={2}
                dot={{ r: 3 }}
                name="Avg Latency (ms)"
              />
            </ComposedChart>
          </ResponsiveContainer>
        ) : (
          <EmptyChart message="No route performance data available." />
        )}
      </ChartCard>
    </div>
  );
}
