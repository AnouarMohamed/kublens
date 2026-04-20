/**
 * Cluster switching hook with optimistic UI messaging.
 */
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { api } from "../../lib/api";
import type { ClusterContextList } from "../../types";

/**
 * Input contract for {@link useClusterSwitcher}.
 */
interface UseClusterSwitcherInput {
  clusterContexts: ClusterContextList | null;
  setClusterContexts: Dispatch<SetStateAction<ClusterContextList | null>>;
  onMessage: (message: string) => void;
}

/**
 * Handles selecting a new active cluster and exposing view-refresh triggers.
 *
 * @param input - Current contexts, setter, and transient message callback.
 * @returns Refresh key, pending state, and selection handler.
 */
export function useClusterSwitcher({ clusterContexts, setClusterContexts, onMessage }: UseClusterSwitcherInput) {
  const [clusterRefreshKey, setClusterRefreshKey] = useState(0);
  const [isSwitchingCluster, setIsSwitchingCluster] = useState(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const refreshCluster = () => {
    setClusterRefreshKey((value) => value + 1);
  };

  const selectCluster = async (nextCluster: string) => {
    if (!clusterContexts || nextCluster === clusterContexts.selected) {
      return;
    }

    setIsSwitchingCluster(true);
    try {
      const response = await api.selectCluster(nextCluster);
      if (!mountedRef.current) {
        return;
      }
      setClusterContexts((current) =>
        current
          ? {
              ...current,
              selected: response.selected,
            }
          : current,
      );
      refreshCluster();
      onMessage(`Switched to cluster: ${response.selected}`);
    } catch (err) {
      if (mountedRef.current) {
        onMessage(err instanceof Error ? err.message : "Failed to switch cluster");
      }
    } finally {
      if (mountedRef.current) {
        setIsSwitchingCluster(false);
      }
    }
  };

  return {
    clusterRefreshKey,
    isSwitchingCluster,
    refreshCluster,
    selectCluster,
  };
}
