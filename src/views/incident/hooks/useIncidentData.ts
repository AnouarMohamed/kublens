import { useCallback, useEffect, useMemo, useState } from "react";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type {
  Incident,
  IncidentEvidenceBundle,
  IncidentReplay,
  MemoryFixCreateRequest,
  RemediationProposal,
  RunbookStep,
  RunbookStepStatus,
  TimelineEntry,
} from "../../../types";
import { deriveNextRunbookAction, formatKind, type TimelineFilter } from "../utils";

type IncidentStatusFilter = "all" | Incident["status"];
type IncidentSeverityFilter = "all" | "critical" | "warning";

interface RunbookStats {
  total: number;
  done: number;
  skipped: number;
  inProgress: number;
  pending: number;
  completionPercent: number;
}

interface IncidentStats {
  total: number;
  open: number;
  criticalOpen: number;
  resolved: number;
}

/**
 * State and actions for the incident commander view.
 */
interface UseIncidentDataResult {
  canRead: boolean;
  canWrite: boolean;
  incidents: Incident[];
  selected: Incident | null;
  replay: IncidentReplay | null;
  evidence: IncidentEvidenceBundle | null;
  remediations: RemediationProposal[];
  isLoading: boolean;
  isActing: boolean;
  error: string | null;
  message: string | null;
  fixPromptDismissed: boolean;
  fixForm: MemoryFixCreateRequest | null;
  statusFilter: IncidentStatusFilter;
  severityFilter: IncidentSeverityFilter;
  searchQuery: string;
  timelineFilter: TimelineFilter;
  associatedExecutedRemediations: RemediationProposal[];
  incidentStats: IncidentStats;
  filteredIncidents: Incident[];
  runbookStats: RunbookStats | null;
  nextRunbookAction: { step: RunbookStep; target: RunbookStepStatus; label: string } | null;
  timelineEntries: TimelineEntry[];
  canResolve: boolean;
  setStatusFilter: (value: IncidentStatusFilter) => void;
  setSeverityFilter: (value: IncidentSeverityFilter) => void;
  setSearchQuery: (value: string) => void;
  setTimelineFilter: (value: TimelineFilter) => void;
  clearSelected: () => void;
  updateFixFormField: (field: keyof MemoryFixCreateRequest, value: string) => void;
  dismissFixPrompt: () => void;
  refreshIncidents: () => Promise<void>;
  refreshIncidentArtifacts: () => Promise<void>;
  loadIncidentDetail: (id: string) => Promise<void>;
  triggerIncident: () => Promise<void>;
  applyStepStatus: (step: RunbookStep, target: RunbookStepStatus) => Promise<void>;
  resolveIncident: () => Promise<void>;
  generatePostmortem: () => Promise<void>;
  saveFix: () => Promise<void>;
  copyEvidenceMarkdown: () => Promise<void>;
}

/**
 * Manages incident list/detail state, runbook actions, and fix-recording workflow.
 *
 * @returns Incident view state and command handlers.
 */
export function useIncidentData(): UseIncidentDataResult {
  const { can, isLoading: authLoading } = useAuthSession();
  const canRead = can("read");
  const canWrite = can("write");

  const [selected, setSelected] = useState<Incident | null>(null);
  const [replay, setReplay] = useState<IncidentReplay | null>(null);
  const [evidence, setEvidence] = useState<IncidentEvidenceBundle | null>(null);
  const [isActing, setIsActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [fixPromptDismissed, setFixPromptDismissed] = useState(false);
  const [fixForm, setFixForm] = useState<MemoryFixCreateRequest | null>(null);
  const [statusFilter, setStatusFilterState] = useState<IncidentStatusFilter>("all");
  const [severityFilter, setSeverityFilterState] = useState<IncidentSeverityFilter>("all");
  const [searchQuery, setSearchQueryState] = useState("");
  const [timelineFilter, setTimelineFilterState] = useState<TimelineFilter>("all");

  const setStatusFilter = useCallback((value: IncidentStatusFilter) => {
    setStatusFilterState(value);
  }, []);

  const setSeverityFilter = useCallback((value: IncidentSeverityFilter) => {
    setSeverityFilterState(value);
  }, []);

  const setSearchQuery = useCallback((value: string) => {
    setSearchQueryState(value);
  }, []);

  const setTimelineFilter = useCallback((value: TimelineFilter) => {
    setTimelineFilterState(value);
  }, []);

  const clearSelected = useCallback(() => {
    setSelected(null);
    setReplay(null);
    setEvidence(null);
  }, []);

  const updateFixFormField = useCallback((field: keyof MemoryFixCreateRequest, value: string) => {
    setFixForm((current) => (current ? { ...current, [field]: value } : null));
  }, []);

  const dismissFixPrompt = useCallback(() => {
    setFixForm(null);
    setFixPromptDismissed(true);
  }, []);

  const loadIncidentRows = useCallback((signal: AbortSignal) => api.listIncidents(signal), []);

  const {
    data: incidents,
    isLoading,
    error: incidentLoadError,
    load: loadIncidents,
    updateData: updateIncidents,
  } = useAsyncResource<Incident[]>({
    loader: loadIncidentRows,
    fallbackError: "Failed to load incidents",
    initialData: [],
    enabled: !authLoading && canRead,
    disabledData: [],
    disabledError: authLoading ? null : "Authenticate to view incidents.",
  });

  const loadRemediationRows = useCallback((signal: AbortSignal) => api.listRemediation(signal), []);

  const { data: remediations, load: loadRemediations } = useAsyncResource<RemediationProposal[]>({
    loader: loadRemediationRows,
    fallbackError: "Failed to load remediation proposals",
    initialData: [],
    autoLoad: false,
    enabled: !authLoading && canRead,
    disabledData: [],
  });

  const error = actionError ?? incidentLoadError;

  const refreshIncidents = useCallback(async () => {
    setActionError(null);
    await loadIncidents();
  }, [loadIncidents]);

  const loadIncidentArtifacts = useCallback(async (id: string) => {
    const [replayResult, evidenceResult] = await Promise.allSettled([
      api.getIncidentReplay(id),
      api.getIncidentEvidence(id),
    ]);
    if (replayResult.status === "fulfilled") {
      setReplay(replayResult.value);
    } else {
      setReplay(null);
    }
    if (evidenceResult.status === "fulfilled") {
      setEvidence(evidenceResult.value);
    } else {
      setEvidence(null);
    }
  }, []);

  const refreshIncidentArtifacts = useCallback(async () => {
    if (!selected) {
      return;
    }
    await loadIncidentArtifacts(selected.id);
  }, [loadIncidentArtifacts, selected]);

  const loadIncidentDetail = useCallback(async (id: string) => {
    setIsActing(true);
    try {
      const [incidentResult, replayResult, evidenceResult] = await Promise.allSettled([
        api.getIncident(id),
        api.getIncidentReplay(id),
        api.getIncidentEvidence(id),
      ]);

      if (incidentResult.status !== "fulfilled") {
        throw incidentResult.reason;
      }

      setSelected(incidentResult.value);
      setReplay(replayResult.status === "fulfilled" ? replayResult.value : null);
      setEvidence(evidenceResult.status === "fulfilled" ? evidenceResult.value : null);
      setFixPromptDismissed(false);
      setTimelineFilterState("all");
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to load incident detail");
    } finally {
      setIsActing(false);
    }
  }, []);

  useEffect(() => {
    if (authLoading || canRead) {
      return;
    }
    setSelected(null);
    setReplay(null);
    setEvidence(null);
  }, [authLoading, canRead]);

  useEffect(() => {
    if (!selected?.id) {
      return;
    }
    const fresh = incidents.find((item) => item.id === selected.id);
    if (fresh) {
      setSelected(fresh);
    }
  }, [incidents, selected?.id]);

  useEffect(() => {
    if (!selected || selected.status !== "resolved") {
      setFixForm(null);
      return;
    }
    void loadRemediations();
  }, [loadRemediations, selected]);

  const associatedExecutedRemediations = useMemo(() => {
    if (!selected) {
      return [];
    }
    const ids = new Set(selected.associatedRemediationIds);
    return remediations.filter((proposal) => ids.has(proposal.id) && proposal.status === "executed");
  }, [selected, remediations]);

  useEffect(() => {
    if (
      !selected ||
      selected.status !== "resolved" ||
      associatedExecutedRemediations.length === 0 ||
      fixPromptDismissed
    ) {
      return;
    }
    const first = associatedExecutedRemediations[0];
    setFixForm({
      incidentId: selected.id,
      proposalId: first.id,
      title: `${formatKind(first.kind)} fix for ${first.resource}`,
      description: first.executionResult || first.reason,
      resource: first.namespace ? `${first.namespace}/${first.resource}` : first.resource,
      kind: first.kind,
    });
  }, [associatedExecutedRemediations, fixPromptDismissed, selected]);

  const triggerIncident = useCallback(async () => {
    if (!canRead) {
      return;
    }
    setIsActing(true);
    try {
      const created = await api.createIncident();
      setMessage(`Incident ${created.id} created.`);
      await refreshIncidents();
      await loadIncidentDetail(created.id);
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to trigger incident");
    } finally {
      setIsActing(false);
    }
  }, [canRead, loadIncidentDetail, refreshIncidents]);

  const applyStepStatus = useCallback(
    async (step: RunbookStep, target: RunbookStepStatus) => {
      if (!selected || !canWrite) {
        return;
      }
      setIsActing(true);
      try {
        const updated = await api.updateIncidentStep(selected.id, step.id, { status: target });
        setSelected(updated);
        updateIncidents((current) => current.map((incident) => (incident.id === updated.id ? updated : incident)));
        await loadIncidentArtifacts(updated.id);
        setMessage(`Step ${step.id} updated to ${target}.`);
        await refreshIncidents();
        setActionError(null);
      } catch (err) {
        setActionError(err instanceof Error ? err.message : "Failed to update step");
      } finally {
        setIsActing(false);
      }
    },
    [canWrite, loadIncidentArtifacts, refreshIncidents, selected, updateIncidents],
  );

  const resolveIncident = useCallback(async () => {
    if (!selected || !canWrite) {
      return;
    }
    setIsActing(true);
    try {
      const updated = await api.resolveIncident(selected.id);
      setSelected(updated);
      updateIncidents((current) => current.map((incident) => (incident.id === updated.id ? updated : incident)));
      await loadIncidentArtifacts(updated.id);
      setMessage(`Incident ${updated.id} resolved.`);
      await refreshIncidents();
      await loadRemediations();
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to resolve incident");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, loadIncidentArtifacts, loadRemediations, refreshIncidents, selected, updateIncidents]);

  const generatePostmortem = useCallback(async () => {
    if (!selected || !canWrite) {
      return;
    }
    setIsActing(true);
    try {
      const created = await api.generatePostmortem(selected.id);
      await loadIncidentArtifacts(selected.id);
      setMessage(`Postmortem ${created.id} generated (${created.method.toUpperCase()}).`);
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to generate postmortem");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, loadIncidentArtifacts, selected]);

  const saveFix = useCallback(async () => {
    if (!fixForm || !canWrite) {
      return;
    }
    setIsActing(true);
    try {
      await api.recordMemoryFix(fixForm);
      setMessage("Fix pattern recorded in cluster memory.");
      setFixForm(null);
      setFixPromptDismissed(true);
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to record fix");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, fixForm]);

  const incidentStats = useMemo(() => {
    const open = incidents.filter((item) => item.status === "open");
    const criticalOpen = open.filter((item) => item.severity === "critical");
    return {
      total: incidents.length,
      open: open.length,
      criticalOpen: criticalOpen.length,
      resolved: incidents.length - open.length,
    };
  }, [incidents]);

  const filteredIncidents = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return incidents.filter((incident) => {
      if (statusFilter !== "all" && incident.status !== statusFilter) {
        return false;
      }
      if (severityFilter !== "all" && incident.severity !== severityFilter) {
        return false;
      }
      if (query === "") {
        return true;
      }
      return `${incident.id} ${incident.title} ${incident.summary} ${incident.affectedResources.join(" ")}`
        .toLowerCase()
        .includes(query);
    });
  }, [incidents, searchQuery, severityFilter, statusFilter]);

  const runbookStats = useMemo(() => {
    if (!selected) {
      return null;
    }
    const total = selected.runbook.length;
    const done = selected.runbook.filter((step) => step.status === "done").length;
    const skipped = selected.runbook.filter((step) => step.status === "skipped").length;
    const inProgress = selected.runbook.filter((step) => step.status === "in_progress").length;
    const pending = selected.runbook.filter((step) => step.status === "pending").length;
    const completionPercent = total > 0 ? Math.round(((done + skipped) / total) * 100) : 0;
    return { total, done, skipped, inProgress, pending, completionPercent };
  }, [selected]);

  const nextRunbookAction = useMemo(() => {
    if (!selected) {
      return null;
    }
    return deriveNextRunbookAction(selected.runbook);
  }, [selected]);

  const timelineEntries = useMemo(() => {
    if (!selected) {
      return [];
    }
    if (timelineFilter === "all") {
      return selected.timeline;
    }
    return selected.timeline.filter((entry) => entry.kind === timelineFilter);
  }, [selected, timelineFilter]);

  const canResolve = useMemo(() => {
    if (!selected || selected.status !== "open") {
      return false;
    }
    return selected.runbook.every((step) => step.status === "done" || step.status === "skipped");
  }, [selected]);

  const copyEvidenceMarkdown = useCallback(async () => {
    if (!evidence?.markdown) {
      return;
    }
    try {
      await navigator.clipboard.writeText(evidence.markdown);
      setMessage("Incident evidence bundle copied.");
      setActionError(null);
    } catch {
      setActionError("Failed to copy incident evidence bundle.");
    }
  }, [evidence?.markdown]);

  return {
    canRead,
    canWrite,
    incidents,
    selected,
    replay,
    evidence,
    remediations,
    isLoading,
    isActing,
    error,
    message,
    fixPromptDismissed,
    fixForm,
    statusFilter,
    severityFilter,
    searchQuery,
    timelineFilter,
    associatedExecutedRemediations,
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
  };
}
