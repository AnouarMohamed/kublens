import { IncidentDetailView } from "./components/IncidentDetailView";
import { IncidentListView } from "./components/IncidentListView";
import { useIncidentData } from "./hooks/useIncidentData";

export default function IncidentView() {
  const {
    canRead,
    canWrite,
    selected,
    replay,
    evidence,
    isLoading,
    isActing,
    error,
    message,
    fixForm,
    statusFilter,
    severityFilter,
    searchQuery,
    timelineFilter,
    incidentStats,
    filteredIncidents,
    runbookStats,
    nextRunbookAction,
    timelineEntries,
    canResolve,
    setStatusFilter,
    setSeverityFilter,
    setSearchQuery,
    setTimelineFilter,
    clearSelected,
    updateFixFormField,
    dismissFixPrompt,
    refreshIncidents,
    refreshIncidentArtifacts,
    loadIncidentDetail,
    triggerIncident,
    applyStepStatus,
    resolveIncident,
    generatePostmortem,
    saveFix,
    copyEvidenceMarkdown,
  } = useIncidentData();

  if (selected) {
    return (
      <IncidentDetailView
        selected={selected}
        replay={replay}
        evidence={evidence}
        canWrite={canWrite}
        isActing={isActing}
        message={message}
        error={error}
        runbookStats={runbookStats}
        nextRunbookAction={nextRunbookAction}
        timelineFilter={timelineFilter}
        timelineEntries={timelineEntries}
        fixForm={fixForm}
        canResolve={canResolve}
        onBack={clearSelected}
        onRefresh={() => void loadIncidentDetail(selected.id)}
        onRefreshArtifacts={() => void refreshIncidentArtifacts()}
        onApplyStepStatus={(step, target) => void applyStepStatus(step, target)}
        onResolveIncident={() => void resolveIncident()}
        onGeneratePostmortem={() => void generatePostmortem()}
        onTimelineFilterChange={setTimelineFilter}
        onFixFormFieldChange={updateFixFormField}
        onSaveFix={() => void saveFix()}
        onDismissFixPrompt={dismissFixPrompt}
        onCopyEvidence={() => void copyEvidenceMarkdown()}
      />
    );
  }

  return (
    <IncidentListView
      canRead={canRead}
      isLoading={isLoading}
      isActing={isActing}
      message={message}
      error={error}
      incidentStats={incidentStats}
      filteredIncidents={filteredIncidents}
      searchQuery={searchQuery}
      statusFilter={statusFilter}
      severityFilter={severityFilter}
      onSearchQueryChange={setSearchQuery}
      onStatusFilterChange={setStatusFilter}
      onSeverityFilterChange={setSeverityFilter}
      onRefresh={() => void refreshIncidents()}
      onTriggerIncident={() => void triggerIncident()}
      onViewIncident={(id) => void loadIncidentDetail(id)}
    />
  );
}
