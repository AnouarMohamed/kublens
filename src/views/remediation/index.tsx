import { RemediationFilters } from "./components/RemediationFilters";
import { RemediationGitOpsPanel } from "./components/RemediationGitOpsPanel";
import { RemediationGuidance } from "./components/RemediationGuidance";
import { RemediationHeader } from "./components/RemediationHeader";
import { ExecuteModal, RejectModal } from "./components/RemediationModals";
import { Banner, StatTile } from "./components/RemediationPrimitives";
import { RemediationProposalGrid } from "./components/RemediationProposalGrid";
import { useRemediationData } from "./hooks/useRemediationData";

export default function RemediationView() {
  const {
    canRead,
    canWrite,
    selectedID,
    rejectingID,
    rejectReason,
    executing,
    gitopsArtifact,
    gitopsLoading,
    gitopsError,
    isLoading,
    isActing,
    error,
    message,
    statusFilter,
    riskFilter,
    searchQuery,
    filteredItems,
    selectedProposal,
    queueHead,
    stats,
    setSelectedID,
    setRejectingID,
    setRejectReason,
    setExecuting,
    setStatusFilter,
    setRiskFilter,
    setSearchQuery,
    refresh,
    refreshGitOpsArtifact,
    propose,
    approve,
    approveAndPrepareExecute,
    generateGitOpsArtifact,
    execute,
    reject,
  } = useRemediationData();

  return (
    <div className="space-y-4">
      <RemediationHeader
        canRead={canRead}
        isLoading={isLoading}
        isActing={isActing}
        onRefresh={() => void refresh()}
        onGenerate={() => void propose()}
      />

      {message && <Banner tone="ok" text={message} />}
      {error && <Banner tone="err" text={error} />}

      <RemediationGuidance selectedProposal={selectedProposal} queueHead={queueHead} />

      <section className="grid gap-3 md:grid-cols-6">
        <StatTile label="Total" value={String(stats.total)} tone="neutral" />
        <StatTile label="Proposed" value={String(stats.proposed)} tone="warn" />
        <StatTile label="Approved" value={String(stats.approved)} tone="accent" />
        <StatTile label="Executed" value={String(stats.executed)} tone="good" />
        <StatTile label="Rejected" value={String(stats.rejected)} tone="neutral" />
        <StatTile label="High risk open" value={String(stats.highRiskOpen)} tone="bad" />
      </section>

      <RemediationFilters
        searchQuery={searchQuery}
        statusFilter={statusFilter}
        riskFilter={riskFilter}
        onSearchQueryChange={setSearchQuery}
        onStatusFilterChange={setStatusFilter}
        onRiskFilterChange={setRiskFilter}
      />

      <RemediationProposalGrid
        filteredItems={filteredItems}
        selectedID={selectedID}
        canRead={canRead}
        canWrite={canWrite}
        isActing={isActing}
        isLoading={isLoading}
        onSelectProposal={setSelectedID}
        onApprove={(id) => void approve(id)}
        onApproveAndQueueExecute={(proposal) => void approveAndPrepareExecute(proposal)}
        onRequestGitOps={(proposal) => void generateGitOpsArtifact(proposal)}
        onRequestReject={(id) => {
          setRejectingID(id);
          setRejectReason("");
        }}
        onRequestExecute={setExecuting}
      />

      <RemediationGitOpsPanel
        proposal={selectedProposal}
        artifact={gitopsArtifact}
        gitopsLoading={gitopsLoading}
        gitopsError={gitopsError}
        isActing={isActing}
        canRead={canRead}
        onGenerate={(proposal) => void generateGitOpsArtifact(proposal)}
        onRefresh={(proposalID) => void refreshGitOpsArtifact(proposalID)}
      />

      <RejectModal
        rejectingID={rejectingID}
        rejectReason={rejectReason}
        isActing={isActing}
        onRejectReasonChange={setRejectReason}
        onCancel={() => setRejectingID(null)}
        onConfirm={() => {
          if (!rejectingID) {
            return;
          }
          void reject(rejectingID, rejectReason);
        }}
      />

      <ExecuteModal
        executing={executing}
        canWrite={canWrite}
        isActing={isActing}
        onCancel={() => setExecuting(null)}
        onConfirm={() => {
          if (!executing) {
            return;
          }
          void execute(executing);
        }}
      />
    </div>
  );
}
