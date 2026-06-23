import { useCallback, useEffect, useMemo, useState } from "react";
import { runAsyncAction } from "../../../app/hooks/asyncTask";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { ApiError } from "../../../lib/api";
import { api } from "../../../lib/api";
import type { RemediationGitOpsArtifact, RemediationProposal } from "../../../types";
import { compareProposalPriority, normalizeRisk, type RiskFilter, type StatusFilter } from "../utils";

interface RemediationStats {
  total: number;
  proposed: number;
  approved: number;
  executed: number;
  rejected: number;
  highRiskOpen: number;
}

/**
 * State and actions for the remediation view.
 */
interface UseRemediationDataResult {
  canRead: boolean;
  canWrite: boolean;
  items: RemediationProposal[];
  selectedID: string | null;
  rejectingID: string | null;
  rejectReason: string;
  executing: RemediationProposal | null;
  gitopsArtifact: RemediationGitOpsArtifact | null;
  gitopsLoading: boolean;
  gitopsError: string | null;
  isLoading: boolean;
  isActing: boolean;
  error: string | null;
  message: string | null;
  statusFilter: StatusFilter;
  riskFilter: RiskFilter;
  searchQuery: string;
  sortedItems: RemediationProposal[];
  filteredItems: RemediationProposal[];
  selectedProposal: RemediationProposal | null;
  queueHead: RemediationProposal | null;
  stats: RemediationStats;
  setSelectedID: (id: string | null) => void;
  setRejectingID: (id: string | null) => void;
  setRejectReason: (reason: string) => void;
  setExecuting: (proposal: RemediationProposal | null) => void;
  setStatusFilter: (status: StatusFilter) => void;
  setRiskFilter: (risk: RiskFilter) => void;
  setSearchQuery: (query: string) => void;
  refresh: () => Promise<void>;
  refreshGitOpsArtifact: (proposalID: string) => Promise<void>;
  propose: () => Promise<void>;
  approve: (id: string) => Promise<RemediationProposal | null>;
  approveAndPrepareExecute: (proposal: RemediationProposal) => Promise<void>;
  generateGitOpsArtifact: (proposal: RemediationProposal) => Promise<void>;
  execute: (proposal: RemediationProposal) => Promise<void>;
  reject: (id: string, reason: string) => Promise<void>;
}

/**
 * Manages proposal retrieval, filtering, and remediation action flows.
 *
 * @returns Remediation state and action handlers.
 */
export function useRemediationData(): UseRemediationDataResult {
  const { can, isLoading: authLoading } = useAuthSession();
  const canRead = can("read");
  const canWrite = can("write");

  const [selectedID, setSelectedIDState] = useState<string | null>(null);
  const [rejectingID, setRejectingIDState] = useState<string | null>(null);
  const [rejectReason, setRejectReasonState] = useState("");
  const [executing, setExecutingState] = useState<RemediationProposal | null>(null);
  const [gitopsArtifact, setGitOpsArtifact] = useState<RemediationGitOpsArtifact | null>(null);
  const [gitopsLoading, setGitOpsLoading] = useState(false);
  const [gitopsError, setGitOpsError] = useState<string | null>(null);
  const [isActing, setIsActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [statusFilter, setStatusFilterState] = useState<StatusFilter>("all");
  const [riskFilter, setRiskFilterState] = useState<RiskFilter>("all");
  const [searchQuery, setSearchQueryState] = useState("");

  const setSelectedID = useCallback((id: string | null) => {
    setSelectedIDState(id);
  }, []);

  const setRejectingID = useCallback((id: string | null) => {
    setRejectingIDState(id);
  }, []);

  const setRejectReason = useCallback((reason: string) => {
    setRejectReasonState(reason);
  }, []);

  const setExecuting = useCallback((proposal: RemediationProposal | null) => {
    setExecutingState(proposal);
  }, []);

  const setStatusFilter = useCallback((status: StatusFilter) => {
    setStatusFilterState(status);
  }, []);

  const setRiskFilter = useCallback((risk: RiskFilter) => {
    setRiskFilterState(risk);
  }, []);

  const setSearchQuery = useCallback((query: string) => {
    setSearchQueryState(query);
  }, []);

  const chooseDefaultSelection = useCallback((rows: RemediationProposal[]) => {
    if (rows.length === 0) {
      setSelectedIDState(null);
      return;
    }
    const next = [...rows].sort(compareProposalPriority)[0];
    setSelectedIDState(next.id);
  }, []);

  const loadRemediationRows = useCallback((signal: AbortSignal) => api.listRemediation(signal), []);

  const {
    data: items,
    isLoading,
    error: loadError,
    load: loadRemediation,
    updateData: updateItems,
  } = useAsyncResource<RemediationProposal[]>({
    loader: loadRemediationRows,
    fallbackError: "Failed to load remediation proposals",
    initialData: [],
    enabled: !authLoading && canRead,
    disabledData: [],
    disabledError: authLoading ? null : "Authenticate to view remediation proposals.",
  });

  const error = actionError ?? loadError;

  const refresh = useCallback(async () => {
    setActionError(null);
    await loadRemediation();
  }, [loadRemediation]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const approveID = params.get("approve");
    if (approveID && approveID.trim() !== "") {
      setSelectedIDState(approveID.trim());
    }
  }, []);

  useEffect(() => {
    if (isLoading) {
      return;
    }
    setSelectedIDState((current) =>
      current && items.some((item) => item.id === current) ? current : (items[0]?.id ?? null),
    );
  }, [isLoading, items]);

  const sortedItems = useMemo(() => [...items].sort(compareProposalPriority), [items]);

  const filteredItems = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return sortedItems.filter((item) => {
      if (statusFilter !== "all" && item.status !== statusFilter) {
        return false;
      }
      if (riskFilter !== "all" && normalizeRisk(item.riskLevel) !== riskFilter) {
        return false;
      }
      if (query === "") {
        return true;
      }
      return `${item.id} ${item.kind} ${item.resource} ${item.namespace} ${item.reason} ${item.status}`
        .toLowerCase()
        .includes(query);
    });
  }, [riskFilter, searchQuery, sortedItems, statusFilter]);

  const selectedProposal = useMemo(() => {
    if (!selectedID) {
      return null;
    }
    return items.find((item) => item.id === selectedID) ?? null;
  }, [items, selectedID]);

  useEffect(() => {
    let cancelled = false;

    async function loadGitOpsArtifact(id: string) {
      setGitOpsLoading(true);
      try {
        const artifact = await api.getRemediationGitOpsArtifact(id);
        if (cancelled) {
          return;
        }
        setGitOpsArtifact(artifact);
        setGitOpsError(null);
      } catch (err) {
        if (cancelled) {
          return;
        }
        if (err instanceof ApiError && err.status === 404) {
          setGitOpsArtifact(null);
          setGitOpsError(null);
          return;
        }
        setGitOpsArtifact(null);
        setGitOpsError(err instanceof Error ? err.message : "Failed to load GitOps artifact");
      } finally {
        if (!cancelled) {
          setGitOpsLoading(false);
        }
      }
    }

    if (!canRead || !selectedProposal) {
      setGitOpsArtifact(null);
      setGitOpsError(null);
      setGitOpsLoading(false);
      return () => {
        cancelled = true;
      };
    }

    void loadGitOpsArtifact(selectedProposal.id);
    return () => {
      cancelled = true;
    };
  }, [canRead, selectedProposal]);

  const queueHead = useMemo(
    () => sortedItems.find((item) => item.status === "proposed" || item.status === "approved") ?? null,
    [sortedItems],
  );

  const stats = useMemo(() => {
    const proposed = items.filter((item) => item.status === "proposed").length;
    const approved = items.filter((item) => item.status === "approved").length;
    const executed = items.filter((item) => item.status === "executed").length;
    const rejected = items.filter((item) => item.status === "rejected").length;
    const highRiskOpen = items.filter(
      (item) => normalizeRisk(item.riskLevel) === "high" && (item.status === "proposed" || item.status === "approved"),
    ).length;
    return { total: items.length, proposed, approved, executed, rejected, highRiskOpen };
  }, [items]);

  const refreshGitOpsArtifact = useCallback(
    async (proposalID: string) => {
      if (!canRead || proposalID.trim() === "") {
        return;
      }
      setGitOpsLoading(true);
      try {
        const artifact = await api.getRemediationGitOpsArtifact(proposalID);
        setGitOpsArtifact(artifact);
        setGitOpsError(null);
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) {
          setGitOpsArtifact(null);
          setGitOpsError(null);
          return;
        }
        setGitOpsArtifact(null);
        setGitOpsError(err instanceof Error ? err.message : "Failed to load GitOps artifact");
      } finally {
        setGitOpsLoading(false);
      }
    },
    [canRead],
  );

  const propose = useCallback(async () => {
    await runAsyncAction({
      setBusy: setIsActing,
      setError: setActionError,
      fallbackError: "Failed to generate proposals",
      action: async () => {
        const proposals = await api.proposeRemediation();
        updateItems(proposals);
        chooseDefaultSelection(proposals);
        setMessage(`Generated ${proposals.length} remediation proposal(s).`);
        setActionError(null);
      },
    });
  }, [chooseDefaultSelection, updateItems]);

  const approve = useCallback(
    async (id: string) => {
      if (!canWrite) {
        return null;
      }
      let updated: RemediationProposal | null = null;
      await runAsyncAction({
        setBusy: setIsActing,
        setError: setActionError,
        fallbackError: "Failed to approve proposal",
        action: async () => {
          updated = await api.approveRemediation(id);
          updateItems((current) => current.map((item) => (item.id === id ? updated! : item)));
          setMessage(`Proposal ${id} approved.`);
          setActionError(null);
        },
      });
      return updated;
    },
    [canWrite, updateItems],
  );

  const approveAndPrepareExecute = useCallback(
    async (proposal: RemediationProposal) => {
      const updated = await approve(proposal.id);
      if (!updated) {
        return;
      }
      setExecutingState(updated);
      setMessage(`Proposal ${proposal.id} approved. Confirm execution next.`);
    },
    [approve],
  );

  const generateGitOpsArtifact = useCallback(
    async (proposal: RemediationProposal) => {
      if (!canRead) {
        return;
      }
      await runAsyncAction({
        setBusy: setIsActing,
        setError: setActionError,
        fallbackError: "Failed to generate GitOps artifact",
        action: async () => {
          const artifact = await api.generateRemediationGitOpsArtifact(proposal.id);
          setGitOpsArtifact(artifact);
          setGitOpsError(null);
          setSelectedIDState(proposal.id);
          setMessage(`Prepared GitOps artifact for ${proposal.id}.`);
          setActionError(null);
        },
      });
    },
    [canRead],
  );

  const execute = useCallback(
    async (proposal: RemediationProposal) => {
      if (!canWrite) {
        return;
      }
      await runAsyncAction({
        setBusy: setIsActing,
        setError: setActionError,
        fallbackError: "Failed to execute proposal",
        action: async () => {
          const updated = await api.executeRemediation(proposal.id);
          updateItems((current) => current.map((item) => (item.id === proposal.id ? updated : item)));
          setExecutingState(null);
          setMessage(`Proposal ${proposal.id} executed.`);
          setActionError(null);
        },
      });
    },
    [canWrite, updateItems],
  );

  const reject = useCallback(
    async (id: string, reason: string) => {
      if (!canRead) {
        return;
      }
      await runAsyncAction({
        setBusy: setIsActing,
        setError: setActionError,
        fallbackError: "Failed to reject proposal",
        action: async () => {
          const updated = await api.rejectRemediation(id, { reason });
          updateItems((current) => current.map((item) => (item.id === id ? updated : item)));
          setRejectingIDState(null);
          setRejectReasonState("");
          setMessage(`Proposal ${id} rejected.`);
          setActionError(null);
        },
      });
    },
    [canRead, updateItems],
  );

  return {
    canRead,
    canWrite,
    items,
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
    sortedItems,
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
  };
}
