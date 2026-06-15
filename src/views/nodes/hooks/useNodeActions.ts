import { useCallback, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import { api } from "../../../lib/api";
import type { K8sEvent, NodeDetail, NodeDrainPreview, Pod } from "../../../types";
import { buildAllocatableDropAlert } from "./nodeRuleEngine";
import type { NodeDrainOptions, NodeRuleAlert } from "./nodesTypes";
import { ensureForceDrainReason, parseCPUCapacity, parseMemoryCapacity } from "./nodesUtils";

interface UseNodeActionsParams {
  canRead: boolean;
  canWrite: boolean;
  load: () => Promise<void>;
  reportError: (message: string) => void;
  reportNotice: (message: string) => void;
  setIsBusy: Dispatch<SetStateAction<boolean>>;
}

export function useNodeActions({
  canRead,
  canWrite,
  load,
  reportError,
  reportNotice,
  setIsBusy,
}: UseNodeActionsParams) {
  const [selectedNode, setSelectedNode] = useState<NodeDetail | null>(null);
  const [selectedNodePods, setSelectedNodePods] = useState<Pod[]>([]);
  const [selectedNodeEvents, setSelectedNodeEvents] = useState<K8sEvent[]>([]);
  const [lastDrainPreview, setLastDrainPreview] = useState<NodeDrainPreview | null>(null);
  const [allocatableDropAlerts, setAllocatableDropAlerts] = useState<NodeRuleAlert[]>([]);
  const allocatableSnapshotRef = useRef<Record<string, { cpu: number; memory: number }>>({});

  const loadNodeContext = useCallback(async (name: string) => {
    const [detail, nodePods, nodeEvents] = await Promise.all([
      api.getNodeDetail(name),
      api.getNodePods(name),
      api.getNodeEvents(name),
    ]);

    setSelectedNode(detail);
    setSelectedNodePods(nodePods);
    setSelectedNodeEvents(nodeEvents);

    const currentAllocatable = {
      cpu: parseCPUCapacity(detail.allocatable.cpu),
      memory: parseMemoryCapacity(detail.allocatable.memory),
    };
    const previous = allocatableSnapshotRef.current[name];
    allocatableSnapshotRef.current[name] = currentAllocatable;

    if (previous && previous.cpu > 0 && previous.memory > 0) {
      const cpuDrop = (previous.cpu - currentAllocatable.cpu) / previous.cpu;
      const memoryDrop = (previous.memory - currentAllocatable.memory) / previous.memory;
      const threshold = 0.1;
      if (cpuDrop >= threshold || memoryDrop >= threshold) {
        setAllocatableDropAlerts((state) => {
          if (state.some((alert) => alert.node === name && alert.rule === "allocatable_drop")) {
            return state;
          }
          return [buildAllocatableDropAlert(name, cpuDrop, memoryDrop), ...state];
        });
      }
    }
  }, []);

  const openDetail = useCallback(
    async (name: string) => {
      if (!canRead) {
        reportError("Authenticate to view node details.");
        return;
      }

      setIsBusy(true);
      try {
        setLastDrainPreview(null);
        await loadNodeContext(name);
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to load node details");
      } finally {
        setIsBusy(false);
      }
    },
    [canRead, loadNodeContext, reportError, setIsBusy],
  );

  const cordon = useCallback(
    async (name: string) => {
      if (!canWrite) {
        reportError("Your role does not allow node cordon actions.");
        return;
      }
      if (!window.confirm(`Cordon node ${name}?`)) {
        return;
      }

      setIsBusy(true);
      try {
        const result = await api.cordonNode(name);
        await load();
        if (selectedNode?.name === name) {
          await loadNodeContext(name);
        }
        reportNotice(result.message || `Node ${name} cordoned.`);
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to cordon node");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load, loadNodeContext, reportError, reportNotice, selectedNode?.name, setIsBusy],
  );

  const uncordon = useCallback(
    async (name: string) => {
      if (!canWrite) {
        reportError("Your role does not allow node uncordon actions.");
        return;
      }
      if (!window.confirm(`Uncordon node ${name}?`)) {
        return;
      }

      setIsBusy(true);
      try {
        const result = await api.uncordonNode(name);
        await load();
        if (selectedNode?.name === name) {
          await loadNodeContext(name);
        }
        reportNotice(result.message || `Node ${name} uncordoned.`);
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to uncordon node");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load, loadNodeContext, reportError, reportNotice, selectedNode?.name, setIsBusy],
  );

  const previewDrain = useCallback(
    async (name: string) => {
      if (!canWrite) {
        reportError("Your role does not allow node drain actions.");
        return;
      }
      setIsBusy(true);
      try {
        const preview = await api.previewNodeDrain(name);
        setLastDrainPreview(preview);
        const summary =
          preview.blockers.length > 0
            ? `Drain preview: ${preview.evictable.length} evictable pods, ${preview.blockers.length} blockers.`
            : `Drain preview: ${preview.evictable.length} evictable pods, no blockers.`;
        reportNotice(summary);
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to preview node drain");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, reportError, reportNotice, setIsBusy],
  );

  const drain = useCallback(
    async (name: string, options: NodeDrainOptions = {}) => {
      const force = options.force === true;
      if (!canWrite) {
        reportError("Your role does not allow node drain actions.");
        return;
      }

      setIsBusy(true);
      try {
        const preview = await api.previewNodeDrain(name);
        setLastDrainPreview(preview);

        if (preview.evictable.length === 0) {
          reportNotice("No evictable pods found on this node.");
          return;
        }

        if (preview.blockers.length > 0 && !force) {
          reportNotice(
            "Drain blocked by safety checks. Run force drain from maintenance mode if you accept the risks.",
          );
          return;
        }
        if (!force && !window.confirm(`Drain node ${name}? This will evict ${preview.evictable.length} pods.`)) {
          return;
        }

        const reason = force ? ensureForceDrainReason(name, options.reason) : "";
        if (force && reason === null) {
          reportNotice("Force drain cancelled. A reason is required to continue.");
          return;
        }

        const result = await api.drainNode(name, { force, reason: reason ?? "" });
        await load();
        if (selectedNode?.name === name) {
          await loadNodeContext(name);
        }
        reportNotice(result.message || `Node ${name} drain requested.`);
      } catch (err) {
        reportError(err instanceof Error ? err.message : "Failed to drain node");
      } finally {
        setIsBusy(false);
      }
    },
    [canWrite, load, loadNodeContext, reportError, reportNotice, selectedNode?.name, setIsBusy],
  );

  const clearSelectedNode = useCallback(() => {
    setSelectedNode(null);
    setSelectedNodePods([]);
    setSelectedNodeEvents([]);
  }, []);

  return {
    selectedNode,
    selectedNodePods,
    selectedNodeEvents,
    lastDrainPreview,
    allocatableDropAlerts,
    openDetail,
    cordon,
    uncordon,
    previewDrain,
    drain,
    clearSelectedNode,
  };
}
