import { useCallback, useState } from "react";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type {
  MemoryFixCreateRequest,
  MemoryFixPattern,
  MemoryRunbook,
  MemoryRunbookUpsertRequest,
} from "../../../types";
import { EMPTY_FIX, EMPTY_RUNBOOK, parseList, parseMultiline } from "../utils";

interface UseMemoryDataResult {
  canRead: boolean;
  canWrite: boolean;
  query: string;
  runbooks: MemoryRunbook[];
  fixes: MemoryFixPattern[];
  editingID: string | null;
  runbookForm: MemoryRunbookUpsertRequest;
  fixForm: MemoryFixCreateRequest;
  isLoading: boolean;
  isActing: boolean;
  error: string | null;
  message: string | null;
  setQuery: (value: string) => void;
  updateRunbookForm: (patch: Partial<MemoryRunbookUpsertRequest>) => void;
  updateFixForm: (patch: Partial<MemoryFixCreateRequest>) => void;
  searchRunbooks: () => Promise<void>;
  searchFixes: () => Promise<void>;
  startEditingRunbook: (runbook: MemoryRunbook) => void;
  resetRunbookForm: () => void;
  saveRunbook: () => Promise<void>;
  saveFix: () => Promise<void>;
}

export function useMemoryData(): UseMemoryDataResult {
  const { can, isLoading: authLoading } = useAuthSession();
  const canRead = can("read");
  const canWrite = can("write");

  const [query, setQueryState] = useState("");
  const [runbookSearchQuery, setRunbookSearchQuery] = useState("");
  const [fixSearchQuery, setFixSearchQuery] = useState("");
  const [editingID, setEditingID] = useState<string | null>(null);
  const [runbookForm, setRunbookForm] = useState<MemoryRunbookUpsertRequest>(EMPTY_RUNBOOK);
  const [fixForm, setFixForm] = useState<MemoryFixCreateRequest>(EMPTY_FIX);
  const [isActing, setIsActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const setQuery = useCallback((value: string) => {
    setQueryState(value);
  }, []);

  const updateRunbookForm = useCallback((patch: Partial<MemoryRunbookUpsertRequest>) => {
    setRunbookForm((current) => ({ ...current, ...patch }));
  }, []);

  const updateFixForm = useCallback((patch: Partial<MemoryFixCreateRequest>) => {
    setFixForm((current) => ({ ...current, ...patch }));
  }, []);

  const loadRunbookRows = useCallback(
    (signal: AbortSignal) => api.searchMemoryRunbooks(runbookSearchQuery, signal),
    [runbookSearchQuery],
  );

  const {
    data: runbooks,
    isLoading: runbooksLoading,
    error: runbooksError,
    load: loadRunbooks,
  } = useAsyncResource<MemoryRunbook[]>({
    loader: loadRunbookRows,
    fallbackError: "Failed to load runbooks",
    initialData: [],
    enabled: !authLoading && canRead,
    disabledData: [],
    disabledError: authLoading ? null : "Authenticate to view memory runbooks.",
  });

  const loadFixRows = useCallback(
    (signal: AbortSignal) => api.listMemoryFixes(fixSearchQuery, signal),
    [fixSearchQuery],
  );

  const {
    data: fixes,
    isLoading: fixesLoading,
    error: fixesError,
    load: loadFixes,
  } = useAsyncResource<MemoryFixPattern[]>({
    loader: loadFixRows,
    fallbackError: "Failed to load fix patterns",
    initialData: [],
    enabled: !authLoading && canRead,
    disabledData: [],
  });

  const isLoading = runbooksLoading || fixesLoading;
  const error = actionError ?? runbooksError ?? fixesError;

  const startEditingRunbook = useCallback((runbook: MemoryRunbook) => {
    setEditingID(runbook.id);
    setRunbookForm({
      title: runbook.title,
      tags: runbook.tags,
      description: runbook.description,
      steps: runbook.steps,
    });
  }, []);

  const resetRunbookForm = useCallback(() => {
    setEditingID(null);
    setRunbookForm(EMPTY_RUNBOOK);
  }, []);

  const saveRunbook = useCallback(async () => {
    if (!canWrite) {
      return;
    }

    setIsActing(true);
    try {
      const payload: MemoryRunbookUpsertRequest = {
        title: runbookForm.title.trim(),
        description: runbookForm.description.trim(),
        tags: parseList(runbookForm.tags.join(", ")),
        steps: parseMultiline(runbookForm.steps.join("\n")),
      };

      if (editingID) {
        await api.updateMemoryRunbook(editingID, payload);
        setMessage(`Runbook ${editingID} updated.`);
      } else {
        const created = await api.createMemoryRunbook(payload);
        setMessage(`Runbook ${created.id} created.`);
      }

      resetRunbookForm();
      if (query === runbookSearchQuery) {
        await loadRunbooks();
      } else {
        setRunbookSearchQuery(query);
      }
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to save runbook");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, editingID, loadRunbooks, query, resetRunbookForm, runbookForm, runbookSearchQuery]);

  const saveFix = useCallback(async () => {
    if (!canWrite) {
      return;
    }

    setIsActing(true);
    try {
      const payload: MemoryFixCreateRequest = {
        incidentId: fixForm.incidentId.trim(),
        proposalId: fixForm.proposalId.trim(),
        title: fixForm.title.trim(),
        description: fixForm.description.trim(),
        resource: fixForm.resource.trim(),
        kind: fixForm.kind,
      };

      const created = await api.recordMemoryFix(payload);
      setMessage(`Fix pattern ${created.id} recorded.`);
      setFixForm(EMPTY_FIX);
      if (query === fixSearchQuery) {
        await loadFixes();
      } else {
        setFixSearchQuery(query);
      }
      setActionError(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to record fix");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, fixForm, fixSearchQuery, loadFixes, query]);

  const searchRunbooks = useCallback(async () => {
    setActionError(null);
    if (query === runbookSearchQuery) {
      await loadRunbooks();
      return;
    }
    setRunbookSearchQuery(query);
  }, [loadRunbooks, query, runbookSearchQuery]);

  const searchFixes = useCallback(async () => {
    setActionError(null);
    if (query === fixSearchQuery) {
      await loadFixes();
      return;
    }
    setFixSearchQuery(query);
  }, [fixSearchQuery, loadFixes, query]);

  return {
    canRead,
    canWrite,
    query,
    runbooks,
    fixes,
    editingID,
    runbookForm,
    fixForm,
    isLoading,
    isActing,
    error,
    message,
    setQuery,
    updateRunbookForm,
    updateFixForm,
    searchRunbooks,
    searchFixes,
    startEditingRunbook,
    resetRunbookForm,
    saveRunbook,
    saveFix,
  };
}
