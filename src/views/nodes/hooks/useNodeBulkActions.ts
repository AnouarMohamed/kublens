import { useCallback, useEffect } from "react";
import type { Dispatch, SetStateAction } from "react";
import { api } from "../../../lib/api";
import type { Node } from "../../../types";
import { useNodeSelection } from "./useNodeSelection";
import type { NodeDrainOptions } from "./nodesTypes";
import { ensureForceDrainReason } from "./nodesUtils";

interface UseNodeBulkActionsParams {
  canWrite: boolean;
  nodes: Node[];
  load: () => Promise<void>;
  reportError: (message: string) => void;
  reportNotice: (message: string) => void;
  setIsBusy: Dispatch<SetStateAction<boolean>>;
}

export function useNodeBulkActions({
  canWrite,
  nodes,
  load,
  reportError,
  reportNotice,
  setIsBusy,
}: UseNodeBulkActionsParams) {
  const { selectedNodeNames, setSelectedNodeNames, toggleNodeSelection, toggleSelectAllVisible, clearNodeSelection } =
    useNodeSelection();

  useEffect(() => {
    setSelectedNodeNames((state) => state.filter((name) => nodes.some((node) => node.name === name)));
  }, [nodes, setSelectedNodeNames]);

  const bulkCordon = useCallback(async () => {
    if (!canWrite) {
      reportError("Your role does not allow node cordon actions.");
      return;
    }
    if (selectedNodeNames.length === 0) {
      reportError("Select at least one node for bulk actions.");
      return;
    }
    if (!window.confirm(`Cordon ${selectedNodeNames.length} selected node(s)?`)) {
      return;
    }

    setIsBusy(true);
    try {
      const results = await Promise.all(selectedNodeNames.map((name) => api.cordonNode(name)));
      await load();
      reportNotice(results[0]?.message || `Cordoned ${selectedNodeNames.length} node(s).`);
      setSelectedNodeNames([]);
    } catch (err) {
      reportError(err instanceof Error ? err.message : "Failed to bulk cordon nodes");
    } finally {
      setIsBusy(false);
    }
  }, [canWrite, load, reportError, reportNotice, selectedNodeNames, setIsBusy, setSelectedNodeNames]);

  const bulkUncordon = useCallback(async () => {
    if (!canWrite) {
      reportError("Your role does not allow node uncordon actions.");
      return;
    }
    if (selectedNodeNames.length === 0) {
      reportError("Select at least one node for bulk actions.");
      return;
    }
    if (!window.confirm(`Uncordon ${selectedNodeNames.length} selected node(s)?`)) {
      return;
    }

    setIsBusy(true);
    try {
      const results = await Promise.all(selectedNodeNames.map((name) => api.uncordonNode(name)));
      await load();
      reportNotice(results[0]?.message || `Uncordoned ${selectedNodeNames.length} node(s).`);
      setSelectedNodeNames([]);
    } catch (err) {
      reportError(err instanceof Error ? err.message : "Failed to bulk uncordon nodes");
    } finally {
      setIsBusy(false);
    }
  }, [canWrite, load, reportError, reportNotice, selectedNodeNames, setIsBusy, setSelectedNodeNames]);

  const bulkDrain = useCallback(
    async (options: NodeDrainOptions = {}) => {
      const force = options.force === true;
      if (!canWrite) {
        reportError("Your role does not allow node drain actions.");
        return;
      }
      if (selectedNodeNames.length === 0) {
        reportError("Select at least one node for bulk actions.");
        return;
      }
      if (
        !window.confirm(
          `${force ? "Force d" : "D"}rain ${selectedNodeNames.length} selected node(s)? This will evict workloads.`,
        )
      ) {
        return;
      }

      setIsBusy(true);
      try {
        const reason = force ? ensureForceDrainReason("selected nodes", options.reason) : "";
        if (force && reason === null) {
          reportNotice("Force drain cancelled. A reason is required to continue.");
          return;
        }

        const blocked: string[] = [];
        for (const name of selectedNodeNames) {
          const preview = await api.previewNodeDrain(name);
          if (!force && preview.blockers.length > 0) {
            blocked.push(name);
            continue;
          }
          await api.drainNode(name, { force, reason: reason ?? "" });
        }
        await load();
        setSelectedNodeNames([]);
        if (blocked.length > 0) {
          reportNotice(`Skipped ${blocked.length} node(s) due to blockers: ${blocked.join(", ")}`);
        } else {
          reportNotice(`Drain requested for ${selectedNodeNames.length} node(s).`);
        }
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to bulk drain nodes");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load, reportError, reportNotice, selectedNodeNames, setIsBusy, setSelectedNodeNames],
  );

  return {
    selectedNodeNames,
    toggleNodeSelection,
    toggleSelectAllVisible,
    clearNodeSelection,
    bulkCordon,
    bulkUncordon,
    bulkDrain,
  };
}
