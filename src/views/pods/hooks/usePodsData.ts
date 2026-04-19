import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { runReadLoad } from "../../../app/hooks/asyncTask";
import { useStreamRefresh } from "../../../app/hooks/useStreamRefresh";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type { Pod, PodCreateRequest, PodDetail } from "../../../types";

export type PodDetailTab = "specs" | "events" | "describe";

export const POD_STATUSES = ["All", "Running", "Pending", "Failed", "Succeeded", "Unknown"] as const;

export type PodStatusFilter = (typeof POD_STATUSES)[number];

const defaultCreateForm: PodCreateRequest = {
  namespace: "default",
  name: "",
  image: "nginx:latest",
};

const defaultLogTailLines = 100;
const maxLogLines = 500;

/**
 * UI state and actions for the pods view.
 */
interface UsePodsDataResult {
  canRead: boolean;
  canWrite: boolean;
  pods: Pod[];
  filteredPods: Pod[];
  namespaces: string[];
  search: string;
  statusFilter: PodStatusFilter;
  namespaceFilter: string;
  selectedPod: PodDetail | null;
  activeTab: PodDetailTab;
  logLines: string[];
  logPodName: string;
  logStreaming: boolean;
  logError: string | null;
  showCreateForm: boolean;
  createForm: PodCreateRequest;
  confirmingDeleteId: string | null;
  isBusy: boolean;
  isLoading: boolean;
  error: string | null;
  setSearch: (value: string) => void;
  setStatusFilter: (value: PodStatusFilter) => void;
  setNamespaceFilter: (value: string) => void;
  setActiveTab: (tab: PodDetailTab) => void;
  toggleCreateForm: () => void;
  updateCreateFormField: (field: keyof PodCreateRequest, value: string) => void;
  load: () => Promise<void>;
  openDetail: (namespace: string, podName: string) => Promise<void>;
  openLogs: (namespace: string, podName: string, container?: string) => Promise<void>;
  startLogStream: (namespace: string, podName: string, container?: string) => void;
  stopLogStream: () => void;
  closeLogs: () => void;
  createPod: () => Promise<void>;
  restartPod: (namespace: string, podName: string) => Promise<void>;
  requestDelete: (pod: Pod) => Promise<void>;
  clearSelectedPod: () => void;
}

function splitLogText(logs: string): string[] {
  if (logs.trim() === "") {
    return [];
  }
  return logs.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n");
}

function trimLogLines(lines: string[]): string[] {
  if (lines.length <= maxLogLines) {
    return lines;
  }
  return lines.slice(-maxLogLines);
}

/**
 * Manages pod inventory state and operational actions.
 *
 * @returns Pods state and command handlers for rendering and interaction.
 */
export function usePodsData(): UsePodsDataResult {
  const { can, isLoading: authLoading } = useAuthSession();
  const [pods, setPods] = useState<Pod[]>([]);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [search, setSearchState] = useState("");
  const [statusFilter, setStatusFilterState] = useState<PodStatusFilter>("All");
  const [namespaceFilter, setNamespaceFilterState] = useState("All");
  const [selectedPod, setSelectedPod] = useState<PodDetail | null>(null);
  const [activeTab, setActiveTabState] = useState<PodDetailTab>("specs");
  const [logLines, setLogLines] = useState<string[]>([]);
  const [logPodName, setLogPodName] = useState("");
  const [logStreaming, setLogStreaming] = useState(false);
  const [logError, setLogError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [createForm, setCreateForm] = useState<PodCreateRequest>(defaultCreateForm);
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<string | null>(null);
  const [isBusy, setIsBusy] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const canRead = can("read");
  const canWrite = can("write");
  const logStreamRef = useRef<EventSource | null>(null);
  const logStreamTokenRef = useRef(0);

  const setSearch = useCallback((value: string) => {
    setSearchState(value);
  }, []);

  const setStatusFilter = useCallback((value: PodStatusFilter) => {
    setStatusFilterState(value);
  }, []);

  const setNamespaceFilter = useCallback((value: string) => {
    setNamespaceFilterState(value);
  }, []);

  const setActiveTab = useCallback((tab: PodDetailTab) => {
    setActiveTabState(tab);
  }, []);

  const toggleCreateForm = useCallback(() => {
    setShowCreateForm((value) => !value);
  }, []);

  const updateCreateFormField = useCallback((field: keyof PodCreateRequest, value: string) => {
    setCreateForm((state) => ({ ...state, [field]: value }));
  }, []);

  const stopLogStream = useCallback(() => {
    logStreamTokenRef.current += 1;
    logStreamRef.current?.close();
    logStreamRef.current = null;
    setLogStreaming(false);
  }, []);

  const load = useCallback(async () => {
    await runReadLoad({
      canRead,
      deniedMessage: "Authenticate to view pod data.",
      fallbackError: "Failed to load pods",
      setIsLoading,
      setError,
      onDenied: () => {
        setPods([]);
        setNamespaces([]);
      },
      load: async () => {
        const [podRows, namespaceRows] = await Promise.all([api.getPods(), api.getNamespaces()]);
        setPods(podRows);
        setNamespaces(namespaceRows);
        setConfirmingDeleteId(null);
      },
    });
  }, [canRead]);

  useEffect(() => {
    if (authLoading) {
      return;
    }
    void load();
  }, [authLoading, load]);

  useEffect(() => {
    return () => {
      stopLogStream();
    };
  }, [stopLogStream]);

  useStreamRefresh({
    enabled: canRead,
    eventTypes: ["pod_update", "pod_restart", "pod_failed", "pod_pending", "pod_deleted"],
    onEvent: () => {
      void load();
    },
  });

  const filteredPods = useMemo(() => {
    const query = search.trim().toLowerCase();
    return pods.filter((pod) => {
      const matchesSearch = query === "" || `${pod.name} ${pod.namespace}`.toLowerCase().includes(query);
      const matchesStatus = statusFilter === "All" || pod.status === statusFilter;
      const matchesNamespace = namespaceFilter === "All" || pod.namespace === namespaceFilter;
      return matchesSearch && matchesStatus && matchesNamespace;
    });
  }, [namespaceFilter, pods, search, statusFilter]);

  const openDetail = useCallback(
    async (namespace: string, podName: string) => {
      if (!canRead) {
        setError("Authenticate to view pod details.");
        return;
      }

      setIsBusy(true);
      try {
        const [detail, events, describe] = await Promise.all([
          api.getPodDetail(namespace, podName),
          api.getPodEvents(namespace, podName),
          api
            .getPodDescribe(namespace, podName)
            .catch((err) => (err instanceof Error ? `Describe failed: ${err.message}` : "Describe failed")),
        ]);
        setSelectedPod({ ...detail, events, describe });
        setActiveTabState("specs");
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load pod details");
      } finally {
        setIsBusy(false);
      }
    },
    [canRead],
  );

  const openLogs = useCallback(
    async (namespace: string, podName: string, container?: string) => {
      if (!canRead) {
        setError("Authenticate to view pod logs.");
        return;
      }

      stopLogStream();
      setLogError(null);
      setConfirmingDeleteId(null);
      setIsBusy(true);
      try {
        const logs = await api.getPodLogs(namespace, podName, defaultLogTailLines, container);
        setLogPodName(`${namespace}/${podName}`);
        setLogLines(trimLogLines(splitLogText(logs)));
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load pod logs");
      } finally {
        setIsBusy(false);
      }
    },
    [canRead, stopLogStream],
  );

  const startLogStream = useCallback(
    (namespace: string, podName: string, container?: string) => {
      if (!canRead) {
        setError("Authenticate to view pod logs.");
        return;
      }

      if (typeof EventSource === "undefined") {
        void openLogs(namespace, podName, container);
        return;
      }

      stopLogStream();
      const streamToken = logStreamTokenRef.current + 1;
      logStreamTokenRef.current = streamToken;

      const source = new EventSource(api.getPodLogStreamURL(namespace, podName, defaultLogTailLines, container, true));
      logStreamRef.current = source;

      setLogPodName(`${namespace}/${podName}`);
      setLogLines([]);
      setLogStreaming(true);
      setLogError(null);
      setConfirmingDeleteId(null);
      setError(null);

      source.onmessage = (event) => {
        if (logStreamTokenRef.current !== streamToken) {
          return;
        }

        if (event.data === "[stream-timeout]") {
          if (logStreamRef.current === source) {
            logStreamRef.current = null;
          }
          logStreamTokenRef.current += 1;
          source.close();
          setLogStreaming(false);
          setLogError("Log stream reached the 10-minute limit.");
          return;
        }

        setLogLines((current) => trimLogLines([...current, event.data]));
      };

      source.onerror = () => {
        if (logStreamTokenRef.current !== streamToken) {
          return;
        }

        if (logStreamRef.current === source) {
          logStreamRef.current = null;
        }
        logStreamTokenRef.current += 1;
        source.close();
        setLogStreaming(false);
        setLogError((current) => current ?? "Log stream disconnected.");
      };
    },
    [canRead, openLogs, stopLogStream],
  );

  const closeLogs = useCallback(() => {
    stopLogStream();
    setLogLines([]);
    setLogPodName("");
    setLogError(null);
  }, [stopLogStream]);

  const createPod = useCallback(async () => {
    if (!canWrite) {
      setError("Your role does not allow pod creation.");
      return;
    }
    if (createForm.name.trim() === "") {
      setError("Pod name is required");
      return;
    }

    setIsBusy(true);
    try {
      await api.createPod({
        namespace: createForm.namespace.trim() || "default",
        name: createForm.name.trim(),
        image: createForm.image.trim() || "nginx:latest",
      });
      setCreateForm(defaultCreateForm);
      setShowCreateForm(false);
      await load();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create pod");
    } finally {
      setIsBusy(false);
    }
  }, [canWrite, createForm.image, createForm.name, createForm.namespace, load]);

  const restartPod = useCallback(
    async (namespace: string, podName: string) => {
      if (!canWrite) {
        setError("Your role does not allow pod restart.");
        return;
      }
      if (!window.confirm(`Restart pod ${namespace}/${podName}?`)) {
        return;
      }
      setConfirmingDeleteId(null);

      setIsBusy(true);
      try {
        await api.restartPod(namespace, podName);
        await load();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to restart pod");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load],
  );

  const deletePod = useCallback(
    async (namespace: string, podName: string) => {
      if (!canWrite) {
        setError("Your role does not allow pod deletion.");
        return;
      }
      setConfirmingDeleteId(null);

      setIsBusy(true);
      try {
        await api.deletePod(namespace, podName);
        await load();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to delete pod");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load],
  );

  const requestDelete = useCallback(
    async (pod: Pod) => {
      if (confirmingDeleteId != pod.id) {
        setConfirmingDeleteId(pod.id);
        return;
      }
      await deletePod(pod.namespace, pod.name);
    },
    [confirmingDeleteId, deletePod],
  );

  const clearSelectedPod = useCallback(() => {
    setSelectedPod(null);
  }, []);

  return {
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
  };
}
