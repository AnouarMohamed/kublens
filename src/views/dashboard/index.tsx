import { navigateToView } from "../../app/viewNavigation";
import { DashboardCharts } from "./components/DashboardCharts";
import { DashboardHeader } from "./components/DashboardHeader";
import { HealthSnapshotCard, RecentEventsCard, TopRiskPodsCard } from "./components/DashboardInsights";
import { DashboardKpiBand } from "./components/DashboardKpiBand";
import { RiskRail } from "./components/DashboardPrimitives";
import { DiagnosticsNarrative } from "./components/DiagnosticsNarrative";
import { useDashboardData } from "./hooks/useDashboardData";

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

      <DashboardCharts
        podMixData={podMixData}
        nodeUsageBars={nodeUsageBars}
        nodeCPUTrend={nodeCPUTrend}
        eventReasonBars={eventReasonBars}
        restartHotspots={restartHotspots}
        apiStatusMix={apiStatusMix}
        onOpenPods={openPods}
        onOpenNodes={openNodes}
        onOpenEvents={openEvents}
        onOpenAudit={openAudit}
      />

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
