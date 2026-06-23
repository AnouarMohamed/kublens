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
import type { RAGTelemetry } from "../../../types";
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
                formatter={(value, name) => {
                  const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                  const label = name === "cpu" ? "CPU" : "Memory";
                  return [`${numeric.toFixed(1)}%`, label] as [string, string];
                }}
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
                formatter={(value) => {
                  const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                  return [`${numeric.toFixed(1)}%`, "Avg CPU"] as [string, string];
                }}
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
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                formatter={(value) => {
                  const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                  return [Math.round(numeric), "Pods"] as [number, string];
                }}
              />
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
            <Tooltip
              contentStyle={TOOLTIP_STYLE}
              formatter={(value) => {
                const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                return [Math.round(numeric), "Pods"] as [number, string];
              }}
            />
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
                  const numeric = coerceNumber(Array.isArray(value) ? value[0] : value);
                  const payload = (item.payload ?? {}) as { cpuMilli?: number; memMi?: number };
                  if (name === "score") {
                    return [
                      `${numeric.toFixed(1)}%`,
                      `Pressure (CPU ${payload.cpuMilli ?? 0}m | Mem ${payload.memMi ?? 0}Mi)`,
                    ] as [string, string];
                  }
                  return [numeric, String(name ?? "")] as [number, string];
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

interface AssistantQualityTabPanelProps {
  ragTelemetry: RAGTelemetry | null;
  ragEmptyRate: number;
  ragFeedbackBalance: number;
}

export function AssistantQualityTabPanel({
  ragTelemetry,
  ragEmptyRate,
  ragFeedbackBalance,
}: AssistantQualityTabPanelProps) {
  const hitRate = (ragTelemetry?.hitRate ?? 0) * 100;
  const averageResults = ragTelemetry?.averageResults ?? 0;
  const recentQueries = ragTelemetry?.recentQueries ?? [];
  const topFeedbackDocs = ragTelemetry?.topFeedbackDocs ?? [];

  return (
    <div className="mt-4 space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
        <QualityMetric
          label="Retrieval Hit Rate"
          value={`${hitRate.toFixed(1)}%`}
          detail={`${ragTelemetry?.totalQueries ?? 0} traced queries`}
        />
        <QualityMetric
          label="Empty Result Rate"
          value={`${ragEmptyRate.toFixed(1)}%`}
          detail={`${ragTelemetry?.emptyResults ?? 0} empty retrievals`}
        />
        <QualityMetric
          label="Avg References"
          value={averageResults.toFixed(1)}
          detail={ragTelemetry?.enabled ? "docs retriever enabled" : "docs retriever disabled"}
        />
        <QualityMetric
          label="Positive Feedback"
          value={`${ragFeedbackBalance.toFixed(1)}%`}
          detail={`${ragTelemetry?.feedbackSignals ?? 0} feedback signals`}
        />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1.35fr_0.65fr] gap-4">
        <ChartCard title="Recent Retrieval Traces">
          {recentQueries.length > 0 ? (
            <div className="divide-y divide-zinc-800">
              {recentQueries.slice(0, 8).map((query) => (
                <div key={`${query.timestamp}-${query.query}`} className="py-3 first:pt-0 last:pb-0">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="max-w-[72ch] text-sm font-medium text-zinc-100">{query.query}</p>
                    <span className="rounded-md border border-zinc-700 bg-zinc-800 px-2 py-1 text-[11px] text-zinc-300">
                      {query.durationMs.toFixed(1)}ms
                    </span>
                  </div>
                  <div className="mt-2 grid grid-cols-2 gap-2 text-xs text-zinc-400 md:grid-cols-4">
                    <TraceFact label="Results" value={String(query.resultCount)} />
                    <TraceFact label="Candidates" value={String(query.candidateCount)} />
                    <TraceFact label="Semantic" value={query.usedSemantic ? "On" : "Off"} />
                    <TraceFact label="Terms" value={query.queryTerms.slice(0, 4).join(", ") || "None"} />
                  </div>
                  {query.topResults.length > 0 && (
                    <div className="mt-3 space-y-2">
                      {query.topResults.slice(0, 3).map((result) => (
                        <div
                          key={`${query.timestamp}-${result.url}`}
                          className="grid gap-2 text-xs md:grid-cols-[minmax(0,1fr)_88px]"
                        >
                          <div className="min-w-0">
                            <p className="truncate font-medium text-zinc-200">{result.title}</p>
                            <p className="truncate text-zinc-500">{result.source || result.url}</p>
                          </div>
                          <div className="text-right font-mono text-zinc-300">
                            {(result.finalScore * 100).toFixed(1)}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <EmptyChart message="No assistant retrieval traces available yet." />
          )}
        </ChartCard>

        <ChartCard title="Reference Feedback">
          {topFeedbackDocs.length > 0 ? (
            <div className="divide-y divide-zinc-800">
              {topFeedbackDocs.slice(0, 8).map((doc) => (
                <div key={doc.url} className="py-3 first:pt-0 last:pb-0">
                  <p className="truncate text-sm font-medium text-zinc-100">{doc.url}</p>
                  <div className="mt-2 grid grid-cols-3 gap-2 text-xs">
                    <TraceFact label="Helpful" value={String(doc.helpful)} />
                    <TraceFact label="Not Helpful" value={String(doc.notHelpful)} />
                    <TraceFact label="Net" value={String(doc.netScore)} />
                  </div>
                  <p className="mt-2 text-[11px] text-zinc-500">{formatTelemetryTime(doc.updatedAt)}</p>
                </div>
              ))}
            </div>
          ) : (
            <EmptyChart message="No reference feedback has been recorded." />
          )}
        </ChartCard>
      </div>
    </div>
  );
}

function QualityMetric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-xl border border-zinc-700 bg-zinc-900 px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold tracking-tight text-zinc-100">{value}</p>
      <p className="mt-1 text-xs text-zinc-400">{detail}</p>
    </div>
  );
}

function TraceFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-[10px] font-semibold uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="truncate text-zinc-300">{value}</p>
    </div>
  );
}

function formatTelemetryTime(value: string): string {
  if (value.trim() === "") {
    return "No timestamp";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}
