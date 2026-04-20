import type { RemediationGitOpsArtifact, RemediationProposal } from "../../../types";
import { displayResource, formatTimestamp } from "../utils";

interface RemediationGitOpsPanelProps {
  proposal: RemediationProposal | null;
  artifact: RemediationGitOpsArtifact | null;
  gitopsLoading: boolean;
  gitopsError: string | null;
  isActing: boolean;
  canRead: boolean;
  onGenerate: (proposal: RemediationProposal) => void;
  onRefresh: (proposalID: string) => void;
}

export function RemediationGitOpsPanel({
  proposal,
  artifact,
  gitopsLoading,
  gitopsError,
  isActing,
  canRead,
  onGenerate,
  onRefresh,
}: RemediationGitOpsPanelProps) {
  if (!proposal) {
    return null;
  }

  return (
    <section className="surface p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] uppercase tracking-[0.22em] text-zinc-500">GitOps Mode</p>
          <h3 className="mt-2 text-lg font-semibold text-zinc-100">{displayResource(proposal)}</h3>
          <p className="mt-1 text-sm text-zinc-400">
            Generate a PR-ready artifact for this remediation proposal instead of executing the change directly.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => onRefresh(proposal.id)}
            disabled={!canRead || isActing || gitopsLoading}
            className="btn"
          >
            {gitopsLoading ? "Loading" : "Refresh artifact"}
          </button>
          <button onClick={() => onGenerate(proposal)} disabled={!canRead || isActing} className="btn-primary">
            {artifact ? "Regenerate GitOps" : "Prepare GitOps"}
          </button>
        </div>
      </div>

      {gitopsError && (
        <div className="mt-4 border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-200">{gitopsError}</div>
      )}

      {artifact ? (
        <div className="mt-4 space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <MetaTile label="Support" value={artifact.artifact.supportLevel.replaceAll("_", " ")} />
            <MetaTile label="Strategy" value={artifact.artifact.strategy.replaceAll("_", " ")} />
            <MetaTile label="Branch" value={artifact.artifact.branchName} />
            <MetaTile label="Generated" value={formatTimestamp(artifact.generatedAt)} />
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            <MetaTile label="PR Title" value={artifact.artifact.prTitle} />
            <MetaTile label="Commit Message" value={artifact.artifact.commitMessage} />
          </div>

          <MetaTile label="Target Path" value={artifact.artifact.targetPath} />

          <div className="border border-zinc-700 bg-zinc-950 p-3">
            <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">
              {artifact.artifact.format.toUpperCase()} Artifact
            </p>
            <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-xs leading-6 text-zinc-300">
              {artifact.artifact.artifactBody}
            </pre>
          </div>

          <div>
            <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">Instructions</p>
            <div className="mt-3 space-y-2">
              {artifact.artifact.instructions.map((instruction) => (
                <div key={instruction} className="border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-300">
                  {instruction}
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
        <div className="mt-4 border border-dashed border-zinc-700 bg-zinc-950 px-4 py-5 text-sm text-zinc-400">
          No GitOps artifact has been generated for this proposal yet.
        </div>
      )}
    </section>
  );
}

function MetaTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-zinc-700 bg-zinc-950 px-3 py-2">
      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{label}</p>
      <p className="mt-1 text-sm font-semibold text-zinc-100">{value}</p>
    </div>
  );
}
