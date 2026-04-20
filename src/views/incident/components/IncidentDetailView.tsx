import type {
  Incident,
  IncidentEvidenceBundle,
  IncidentReplay,
  MemoryFixCreateRequest,
  RunbookStep,
  RunbookStepStatus,
  TimelineEntry,
} from "../../../types";
import { formatTimestamp, stepIcon, timelineFilters, type TimelineFilter } from "../utils";
import { Banner, StatTile, TimelineCard } from "./IncidentPrimitives";

interface IncidentRunbookStats {
  total: number;
  done: number;
  skipped: number;
  inProgress: number;
  pending: number;
  completionPercent: number;
}

interface IncidentDetailViewProps {
  selected: Incident;
  replay: IncidentReplay | null;
  evidence: IncidentEvidenceBundle | null;
  canWrite: boolean;
  isActing: boolean;
  message: string | null;
  error: string | null;
  runbookStats: IncidentRunbookStats | null;
  nextRunbookAction: { step: RunbookStep; target: RunbookStepStatus; label: string } | null;
  timelineFilter: TimelineFilter;
  timelineEntries: TimelineEntry[];
  fixForm: MemoryFixCreateRequest | null;
  canResolve: boolean;
  onBack: () => void;
  onRefresh: () => void;
  onRefreshArtifacts: () => void;
  onApplyStepStatus: (step: RunbookStep, target: RunbookStepStatus) => void;
  onResolveIncident: () => void;
  onGeneratePostmortem: () => void;
  onTimelineFilterChange: (value: TimelineFilter) => void;
  onFixFormFieldChange: (field: keyof MemoryFixCreateRequest, value: string) => void;
  onSaveFix: () => void;
  onDismissFixPrompt: () => void;
  onCopyEvidence: () => void;
}

export function IncidentDetailView({
  selected,
  replay,
  evidence,
  canWrite,
  isActing,
  message,
  error,
  runbookStats,
  nextRunbookAction,
  timelineFilter,
  timelineEntries,
  fixForm,
  canResolve,
  onBack,
  onRefresh,
  onRefreshArtifacts,
  onApplyStepStatus,
  onResolveIncident,
  onGeneratePostmortem,
  onTimelineFilterChange,
  onFixFormFieldChange,
  onSaveFix,
  onDismissFixPrompt,
  onCopyEvidence,
}: IncidentDetailViewProps) {
  return (
    <div className="space-y-4">
      <header className="panel-head">
        <div>
          <button onClick={onBack} className="btn-sm border-zinc-600">
            Back to Incidents
          </button>
          <h2 className="mt-2 text-2xl font-semibold text-zinc-100 tracking-tight">{selected.title}</h2>
          <p className="text-sm text-zinc-400 mt-1">
            {selected.id} | {selected.severity.toUpperCase()} | {selected.status.toUpperCase()}
          </p>
        </div>
        <div className="flex gap-2">
          {nextRunbookAction && selected.status === "open" && (
            <button
              onClick={() => onApplyStepStatus(nextRunbookAction.step, nextRunbookAction.target)}
              disabled={!canWrite || isActing}
              className="btn-primary"
            >
              {nextRunbookAction.label}
            </button>
          )}
          {selected.status === "open" && (
            <button
              onClick={onResolveIncident}
              disabled={!canWrite || !canResolve || isActing}
              className="btn"
              title={canResolve ? undefined : "Complete or skip all runbook steps first"}
            >
              Resolve Incident
            </button>
          )}
          {selected.status === "resolved" && (
            <button onClick={onGeneratePostmortem} disabled={!canWrite || isActing} className="btn">
              Generate Postmortem
            </button>
          )}
          <button onClick={onRefresh} disabled={isActing} className="btn">
            Refresh
          </button>
        </div>
      </header>

      {message && <Banner tone="ok" text={message} />}
      {error && <Banner tone="err" text={error} />}

      <section className="grid gap-3 md:grid-cols-5">
        <StatTile label="Runbook completion" value={`${runbookStats?.completionPercent ?? 0}%`} tone="accent" />
        <StatTile label="Done" value={String(runbookStats?.done ?? 0)} tone="good" />
        <StatTile label="In progress" value={String(runbookStats?.inProgress ?? 0)} tone="warn" />
        <StatTile label="Pending" value={String(runbookStats?.pending ?? 0)} tone="neutral" />
        <StatTile label="Skipped" value={String(runbookStats?.skipped ?? 0)} tone="neutral" />
      </section>

      <section className="surface p-4">
        <h3 className="text-sm font-semibold text-zinc-100 uppercase tracking-wide">Affected resources</h3>
        <div className="mt-3 flex flex-wrap gap-2">
          {selected.affectedResources.map((resource) => (
            <span
              key={resource}
              className="rounded-md border border-zinc-700 bg-zinc-900/70 px-2 py-1 text-xs text-zinc-300"
            >
              {resource}
            </span>
          ))}
          {selected.affectedResources.length === 0 && (
            <p className="text-sm text-zinc-500">No affected resources listed.</p>
          )}
        </div>
      </section>

      {fixForm && (
        <section className="surface p-4 space-y-3">
          <p className="text-sm font-semibold text-zinc-100">Record this fix for future reference?</p>
          <p className="text-xs text-zinc-400">
            This incident has an executed remediation. Save a reusable fix pattern to cluster memory.
          </p>
          <div className="grid gap-3 md:grid-cols-2">
            <label className="text-xs text-zinc-400">
              Title
              <input
                value={fixForm.title}
                onChange={(event) => onFixFormFieldChange("title", event.target.value)}
                className="field mt-1 w-full"
              />
            </label>
            <label className="text-xs text-zinc-400">
              Resource
              <input
                value={fixForm.resource}
                onChange={(event) => onFixFormFieldChange("resource", event.target.value)}
                className="field mt-1 w-full"
              />
            </label>
          </div>
          <label className="text-xs text-zinc-400 block">
            Description
            <textarea
              value={fixForm.description}
              onChange={(event) => onFixFormFieldChange("description", event.target.value)}
              className="field mt-1 w-full min-h-24"
            />
          </label>
          <div className="flex gap-2">
            <button onClick={onSaveFix} disabled={!canWrite || isActing} className="btn-primary">
              Record Fix
            </button>
            <button onClick={onDismissFixPrompt} className="btn">
              Dismiss
            </button>
          </div>
        </section>
      )}

      <section className="grid gap-4 xl:grid-cols-2">
        <article className="surface p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-zinc-100 uppercase tracking-wide">Incident Replay</h3>
              <p className="mt-1 text-xs text-zinc-400">
                Deterministic playback of the incident timeline from detection to resolution.
              </p>
            </div>
            <button onClick={onRefreshArtifacts} disabled={isActing} className="btn-sm border-zinc-600">
              Refresh Replay
            </button>
          </div>

          {replay ? (
            <>
              <div className="mt-3 grid gap-3 sm:grid-cols-3">
                <StatTile label="Frames" value={String(replay.frames.length)} tone="neutral" />
                <StatTile label="Window" value={replay.duration} tone="accent" />
                <StatTile
                  label="Status"
                  value={replay.status.toUpperCase()}
                  tone={replay.status === "resolved" ? "good" : "warn"}
                />
              </div>
              <div className="mt-3 space-y-3">
                {replay.frames.map((frame, index) => (
                  <div key={`${frame.timestamp}-${index}`} className="border border-zinc-700 bg-zinc-900/70 px-3 py-2">
                    <div className="flex items-center justify-between gap-3">
                      <p className="text-sm font-semibold text-zinc-100">
                        T+{frame.offsetMinutes}m · {frame.kind}
                      </p>
                      <span className="text-[11px] uppercase text-zinc-500">{frame.source}</span>
                    </div>
                    <p className="mt-1 text-sm text-zinc-300">{frame.summary}</p>
                    <p className="mt-1 text-xs text-zinc-500">
                      {formatTimestamp(frame.timestamp)}
                      {frame.resource ? ` · ${frame.resource}` : ""}
                    </p>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <p className="mt-3 text-sm text-zinc-500">Replay data is unavailable for this incident.</p>
          )}
        </article>

        <article className="surface p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-zinc-100 uppercase tracking-wide">Evidence Bundle</h3>
              <p className="mt-1 text-xs text-zinc-400">
                Audit-backed snapshot of findings, actions, and linked remediation context.
              </p>
            </div>
            <button onClick={onCopyEvidence} disabled={!evidence || isActing} className="btn-sm border-zinc-600">
              Copy Bundle
            </button>
          </div>

          {evidence ? (
            <>
              <div className="mt-3 grid gap-3 sm:grid-cols-3">
                <StatTile label="Diagnostics" value={String(evidence.diagnostics.length)} tone="bad" />
                <StatTile label="Audit items" value={String(evidence.audit.length)} tone="neutral" />
                <StatTile label="Remediations" value={String(evidence.remediations.length)} tone="accent" />
              </div>

              <div className="mt-3 border border-zinc-700 bg-zinc-900/70 px-3 py-2">
                <p className="text-xs uppercase tracking-[0.18em] text-zinc-500">Bundle Summary</p>
                <p className="mt-2 text-sm text-zinc-300">{evidence.summary}</p>
                <p className="mt-1 text-xs text-zinc-500">Generated {formatTimestamp(evidence.generatedAt)}</p>
              </div>

              <div className="mt-3 space-y-3">
                <EvidenceList
                  title="Recent Audit"
                  items={evidence.audit.slice(-5).map((entry) => ({
                    key: `${entry.id}-${entry.timestamp}`,
                    title: entry.action || `${entry.method} ${entry.path}`,
                    detail: `${formatTimestamp(entry.timestamp)} · ${entry.status}`,
                  }))}
                  emptyText="No related audit entries."
                />
                <EvidenceList
                  title="Linked Remediation"
                  items={evidence.remediations.map((proposal) => ({
                    key: proposal.id,
                    title: `${proposal.namespace ? `${proposal.namespace}/` : ""}${proposal.resource}`,
                    detail: `${proposal.status.toUpperCase()} · ${proposal.reason}`,
                  }))}
                  emptyText="No linked remediations."
                />
                <EvidenceList
                  title="Postmortem"
                  items={
                    evidence.postmortem
                      ? [
                          {
                            key: evidence.postmortem.id,
                            title: evidence.postmortem.rootCause,
                            detail: `${evidence.postmortem.method.toUpperCase()} · ${formatTimestamp(evidence.postmortem.generatedAt)}`,
                          },
                        ]
                      : []
                  }
                  emptyText="Postmortem not generated."
                />
              </div>
            </>
          ) : (
            <p className="mt-3 text-sm text-zinc-500">Evidence bundle data is unavailable for this incident.</p>
          )}
        </article>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <article className="surface p-4">
          <div className="flex items-center justify-between gap-3">
            <h3 className="text-sm font-semibold text-zinc-100 uppercase tracking-wide">Timeline</h3>
            <div className="flex flex-wrap gap-2">
              {timelineFilters.map((item) => (
                <button
                  key={item.value}
                  onClick={() => onTimelineFilterChange(item.value)}
                  className={`btn-sm ${timelineFilter === item.value ? "border-[var(--accent)] bg-[var(--accent-dim)] text-zinc-100" : ""}`}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
          <div className="mt-3 space-y-3">
            {timelineEntries.map((entry, idx) => (
              <TimelineCard key={`${entry.timestamp}-${idx}`} entry={entry} />
            ))}
            {timelineEntries.length === 0 && (
              <p className="text-sm text-zinc-500">No timeline entries for this filter.</p>
            )}
          </div>
        </article>

        <article className="surface p-4">
          <h3 className="text-sm font-semibold text-zinc-100 uppercase tracking-wide">Runbook</h3>
          {runbookStats && (
            <div className="mt-3 rounded-md border border-zinc-700 bg-zinc-900/70 p-2">
              <div className="flex items-center justify-between gap-2 text-xs text-zinc-500">
                <span>
                  Progress: {runbookStats.done + runbookStats.skipped}/{runbookStats.total} complete
                </span>
                <span>{runbookStats.completionPercent}%</span>
              </div>
              <div className="mt-1.5 h-1.5 bg-zinc-700 overflow-hidden">
                <div
                  className="h-full bg-[var(--accent)]"
                  style={{ width: `${Math.max(0, Math.min(100, runbookStats.completionPercent))}%` }}
                />
              </div>
            </div>
          )}
          <div className="mt-3 space-y-3">
            {selected.runbook.map((step) => (
              <div
                key={step.id}
                className={`rounded-md border bg-zinc-900/70 p-3 ${
                  nextRunbookAction?.step.id === step.id ? "border-[var(--accent)]" : "border-zinc-700"
                }`}
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="text-sm font-semibold text-zinc-100">
                    {stepIcon(step.status)} {step.title}
                  </p>
                  <span className="text-[11px] uppercase text-zinc-500">{step.status}</span>
                </div>
                <p className="mt-1 text-sm text-zinc-300">{step.description}</p>
                {step.command && (
                  <pre className="mt-2 overflow-x-auto rounded-md border border-zinc-700 bg-zinc-950 p-2 text-xs text-zinc-200">
                    <code>{step.command}</code>
                  </pre>
                )}
                <div className="mt-2 flex flex-wrap gap-2">
                  {step.status === "pending" && (
                    <button
                      onClick={() => onApplyStepStatus(step, "in_progress")}
                      disabled={!canWrite || isActing}
                      className="btn-sm border-zinc-600"
                    >
                      Start
                    </button>
                  )}
                  {step.status === "in_progress" && (
                    <button
                      onClick={() => onApplyStepStatus(step, "done")}
                      disabled={!canWrite || isActing}
                      className="btn-sm border-zinc-600"
                    >
                      Mark Done
                    </button>
                  )}
                  {(step.status === "pending" || step.status === "in_progress") && !step.mandatory && (
                    <button
                      onClick={() => onApplyStepStatus(step, "skipped")}
                      disabled={!canWrite || isActing}
                      className="btn-sm border-zinc-600"
                    >
                      Skip
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </article>
      </section>
    </div>
  );
}

function EvidenceList({
  title,
  items,
  emptyText,
}: {
  title: string;
  items: Array<{ key: string; title: string; detail: string }>;
  emptyText: string;
}) {
  return (
    <div>
      <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{title}</p>
      <div className="mt-2 space-y-2">
        {items.map((item) => (
          <div key={item.key} className="border border-zinc-700 bg-zinc-950 px-3 py-2">
            <p className="text-sm text-zinc-100">{item.title}</p>
            <p className="mt-1 text-xs text-zinc-500">{item.detail}</p>
          </div>
        ))}
        {items.length === 0 && <p className="text-sm text-zinc-500">{emptyText}</p>}
      </div>
    </div>
  );
}
