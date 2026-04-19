import PodDetailModal from "../../components/pods/PodDetailModal";
import { PodCreateForm } from "./components/PodCreateForm";
import { PodLogsModal } from "./components/PodLogsModal";
import { PodsFilters } from "./components/PodsFilters";
import { PodsHeader } from "./components/PodsHeader";
import { PodsSummary } from "./components/PodsSummary";
import { PodsTable } from "./components/PodsTable";
import { POD_STATUSES, usePodsData } from "./hooks/usePodsData";

export default function Pods() {
  const {
    canRead,
    canWrite,
    pods,
    filteredPods,
    namespaces,
    search,
    statusFilter,
    namespaceFilter,
    selectedPod,
    activeTab,
    logLines,
    logPodName,
    logStreaming,
    logError,
    showCreateForm,
    createForm,
    confirmingDeleteId,
    isBusy,
    isLoading,
    error,
    setSearch,
    setStatusFilter,
    setNamespaceFilter,
    setActiveTab,
    toggleCreateForm,
    updateCreateFormField,
    load,
    openDetail,
    openLogs,
    startLogStream,
    stopLogStream,
    closeLogs,
    createPod,
    restartPod,
    requestDelete,
    clearSelectedPod,
  } = usePodsData();

  return (
    <div className="space-y-5">
      <PodsHeader
        canWrite={canWrite}
        showCreateForm={showCreateForm}
        canRead={canRead}
        isLoading={isLoading}
        isBusy={isBusy}
        onToggleCreateForm={toggleCreateForm}
        onRefresh={() => void load()}
      />

      {showCreateForm && canWrite && (
        <PodCreateForm
          createForm={createForm}
          isBusy={isBusy}
          onFieldChange={updateCreateFormField}
          onSubmit={() => void createPod()}
        />
      )}

      <PodsFilters
        search={search}
        namespaceFilter={namespaceFilter}
        statusFilter={statusFilter}
        namespaces={namespaces}
        statuses={POD_STATUSES}
        onSearchChange={setSearch}
        onNamespaceFilterChange={setNamespaceFilter}
        onStatusFilterChange={setStatusFilter}
      />

      <PodsSummary pods={pods} filteredCount={filteredPods.length} />

      {error && (
        <div className="rounded-md border border-zinc-700 bg-zinc-900/80 px-3 py-2 text-sm text-zinc-200">{error}</div>
      )}

      <PodsTable
        pods={filteredPods}
        canRead={canRead}
        canWrite={canWrite}
        isLoading={isLoading}
        confirmingDeleteId={confirmingDeleteId}
        onOpenDetail={openDetail}
        onOpenLogs={startLogStream}
        onOpenSnapshot={openLogs}
        onRestartPod={restartPod}
        onRequestDelete={requestDelete}
      />

      <PodLogsModal
        logLines={logLines}
        logPodName={logPodName}
        logStreaming={logStreaming}
        logError={logError}
        onStop={stopLogStream}
        onClose={closeLogs}
      />

      <PodDetailModal
        selectedPod={selectedPod}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        onClose={clearSelectedPod}
      />
    </div>
  );
}
