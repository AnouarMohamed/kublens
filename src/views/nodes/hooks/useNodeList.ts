import { useCallback, useEffect, useMemo, useState } from "react";
import { runReadLoad } from "../../../app/hooks/asyncTask";
import { useStreamRefresh } from "../../../app/hooks/useStreamRefresh";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type { K8sEvent, Node, NodeAlertLifecycle } from "../../../types";
import { indexAlertLifecycleByID } from "./nodesUtils";

export function useNodeList() {
  const { can, isLoading: authLoading } = useAuthSession();
  const [nodes, setNodes] = useState<Node[]>([]);
  const [clusterEvents, setClusterEvents] = useState<K8sEvent[]>([]);
  const [alertLifecycleByID, setAlertLifecycleByID] = useState<Record<string, NodeAlertLifecycle>>({});
  const [search, setSearchState] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const canRead = can("read");
  const canWrite = can("write");

  const setSearch = useCallback((value: string) => {
    setSearchState(value);
  }, []);

  const reportError = useCallback((message: string) => {
    setError(message);
    setNotice(null);
  }, []);

  const reportNotice = useCallback((message: string) => {
    setNotice(message);
    setError(null);
  }, []);

  const load = useCallback(async () => {
    await runReadLoad({
      canRead,
      deniedMessage: "Authenticate to view node data.",
      fallbackError: "Failed to load nodes",
      setIsLoading,
      setError,
      onDenied: () => {
        setNodes([]);
        setClusterEvents([]);
        setAlertLifecycleByID({});
        setNotice(null);
      },
      load: async () => {
        const [nodeRows, eventRows, lifecycleRows] = await Promise.all([
          api.getNodes(),
          api.getEvents(),
          api.getAlertLifecycle().catch(() => [] as NodeAlertLifecycle[]),
        ]);
        setNodes(nodeRows);
        setClusterEvents(eventRows);
        setAlertLifecycleByID(indexAlertLifecycleByID(lifecycleRows));
        setNotice(null);
      },
    });
  }, [canRead]);

  useEffect(() => {
    if (authLoading) {
      return;
    }
    void load();
  }, [authLoading, load]);

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
