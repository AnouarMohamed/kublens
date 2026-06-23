import { useCallback, useEffect, useMemo, useState } from "react";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { useStreamRefresh } from "../../../app/hooks/useStreamRefresh";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type { K8sEvent, Node, NodeAlertLifecycle } from "../../../types";
import { indexAlertLifecycleByID } from "./nodesUtils";

interface NodeListPayload {
  nodes: Node[];
  clusterEvents: K8sEvent[];
  alertLifecycleByID: Record<string, NodeAlertLifecycle>;
}

export function useNodeList() {
  const { can, isLoading: authLoading } = useAuthSession();
  const [alertLifecycleByID, setAlertLifecycleByID] = useState<Record<string, NodeAlertLifecycle>>({});
  const [search, setSearchState] = useState("");
  const [actionError, setActionError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const canRead = can("read");
  const canWrite = can("write");

  const setSearch = useCallback((value: string) => {
    setSearchState(value);
  }, []);

  const reportError = useCallback((message: string) => {
    setActionError(message);
    setNotice(null);
  }, []);

  const reportNotice = useCallback((message: string) => {
    setNotice(message);
    setActionError(null);
  }, []);

  const loadNodeList = useCallback(async (signal: AbortSignal): Promise<NodeListPayload> => {
    const [nodeRows, eventRows, lifecycleRows] = await Promise.all([
      api.getNodes(signal),
      api.getEvents(signal),
      api.getAlertLifecycle(signal).catch(() => [] as NodeAlertLifecycle[]),
    ]);

    return {
      nodes: nodeRows,
      clusterEvents: eventRows,
      alertLifecycleByID: indexAlertLifecycleByID(lifecycleRows),
    };
  }, []);

  const {
    data: listPayload,
    isLoading,
    error: loadError,
    load: loadNodes,
  } = useAsyncResource<NodeListPayload>({
    loader: loadNodeList,
    fallbackError: "Failed to load nodes",
    initialData: { nodes: [], clusterEvents: [], alertLifecycleByID: {} },
    enabled: !authLoading && canRead,
    disabledData: { nodes: [], clusterEvents: [], alertLifecycleByID: {} },
    disabledError: authLoading ? null : "Authenticate to view node data.",
  });

  const { nodes, clusterEvents } = listPayload;
  const error = actionError ?? loadError;

  const load = useCallback(async () => {
    setActionError(null);
    setNotice(null);
    await loadNodes();
  }, [loadNodes]);

  useEffect(() => {
    setAlertLifecycleByID(listPayload.alertLifecycleByID);
  }, [listPayload.alertLifecycleByID]);

  useStreamRefresh({
    enabled: canRead,
    eventTypes: ["node_update", "node_not_ready", "node_pressure", "node_deleted"],
    onEvent: () => {
      void load();
    },
  });

  const filteredNodes = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (query === "") {
      return nodes;
    }

    return nodes.filter((node) => `${node.name} ${node.roles} ${node.status}`.toLowerCase().includes(query));
  }, [nodes, search]);

  return {
    canRead,
    canWrite,
    nodes,
    clusterEvents,
    filteredNodes,
    alertLifecycleByID,
    setAlertLifecycleByID,
    search,
    isLoading,
    error,
    notice,
    setSearch,
    load,
    reportError,
    reportNotice,
  };
}
