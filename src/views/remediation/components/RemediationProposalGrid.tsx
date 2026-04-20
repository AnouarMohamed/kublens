import type { RemediationProposal } from "../../../types";
import { displayResource, formatTimestamp, normalizeRisk, riskClass } from "../utils";

interface RemediationProposalGridProps {
  filteredItems: RemediationProposal[];
  selectedID: string | null;
  canRead: boolean;
  canWrite: boolean;
  isActing: boolean;
  isLoading: boolean;
  onSelectProposal: (id: string) => void;
  onApprove: (id: string) => void;
  onApproveAndQueueExecute: (proposal: RemediationProposal) => void;
  onRequestGitOps: (proposal: RemediationProposal) => void;
  onRequestReject: (id: string) => void;
  onRequestExecute: (proposal: RemediationProposal) => void;
}

export function RemediationProposalGrid({
  filteredItems,
  selectedID,
  canRead,
  canWrite,
  isActing,
  isLoading,
  onSelectProposal,
  onApprove,
  onApproveAndQueueExecute,
  onRequestGitOps,
  onRequestReject,
  onRequestExecute,
}: RemediationProposalGridProps) {
  return (
    <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {filteredItems.map((proposal) => {
        const canReject = proposal.status === "proposed" || proposal.status === "approved";
        const risk = normalizeRisk(proposal.riskLevel);
        return (
          <article
            key={proposal.id}
            className={`surface p-4 ${selectedID === proposal.id ? "border-[#3b82f6]/60" : ""}`}
            onClick={() => onSelectProposal(proposal.id)}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="rounded-full border border-zinc-600 px-2 py-0.5 text-[11px] uppercase text-zinc-200">
                {proposal.kind.replaceAll("_", " ")}
              </span>
              <span
                className={`rounded-full border px-2 py-0.5 text-[11px] uppercase ${riskClass(proposal.riskLevel)}`}
              >
                {proposal.riskLevel}
              </span>
            </div>

            <p className="mt-2 text-sm font-semibold text-zinc-100">{displayResource(proposal)}</p>
            <p className="mt-1 text-sm text-zinc-300">{proposal.reason}</p>
            <p className="mt-2 text-xs uppercase text-zinc-500">Status: {proposal.status}</p>
            <p className="text-[11px] text-zinc-500 mt-1">Updated: {formatTimestamp(proposal.updatedAt)}</p>

            <details className="mt-3 rounded-md border border-zinc-700 bg-zinc-900/70 p-2">
              <summary className="cursor-pointer text-xs uppercase tracking-wide text-zinc-500">Dry-run result</summary>
              <p className="mt-2 text-sm text-zinc-300">{proposal.dryRunResult}</p>
            </details>

            {proposal.executionResult && (
              <p className="mt-2 rounded-md border border-zinc-700 bg-zinc-900/70 px-2 py-1 text-xs text-zinc-300">
                Executed: {proposal.executionResult}
              </p>
            )}

            <div className="mt-3 flex flex-wrap gap-2">
              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onApprove(proposal.id);
                }}
                disabled={!canWrite || isActing || proposal.status !== "proposed"}
                className="btn-sm border-zinc-600"
              >
                Approve
              </button>

              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onRequestGitOps(proposal);
                }}
                disabled={!canRead || isActing}
                className="btn-sm border-zinc-600"
              >
                Prepare GitOps
              </button>

              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onRequestReject(proposal.id);
                }}
                disabled={!canRead || isActing || !canReject}
                className="btn-sm border-zinc-600"
              >
                Reject
              </button>

              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onRequestExecute(proposal);
                }}
                disabled={!canWrite || isActing || proposal.status !== "approved"}
                className="btn-sm border-zinc-600"
              >
                Execute
              </button>

              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onApproveAndQueueExecute(proposal);
                }}
                disabled={!canWrite || isActing || proposal.status !== "proposed" || risk !== "low"}
                className="btn-sm border-zinc-600"
                title={risk === "low" ? "Fast path for low-risk proposals" : "Only available for low-risk proposals"}
              >
                Approve and Queue Execute
              </button>
            </div>
          </article>
        );
      })}

      {!isLoading && filteredItems.length === 0 && (
        <article className="surface p-4 md:col-span-2 xl:col-span-3">
          <p className="text-sm text-zinc-500">No remediation proposals match your current filters.</p>
        </article>
      )}
    </section>
  );
}
