import { useMemo, useState } from "react";
import type { K8sEvent, Node, NodeDetail, NodeDrainPreview, Pod } from "../../../types";
import { deriveNodeRuleAlerts } from "./nodeRuleEngine";
import { useNodeActions } from "./useNodeActions";
import { useNodeAlertActions } from "./useNodeAlertActions";
import { useNodeBulkActions } from "./useNodeBulkActions";
import { useNodeList } from "./useNodeList";
import type { NodeDrainOptions, NodeRuleAlert } from "./nodesTypes";

/**
 * UI state and actions for the nodes view.
 */
interface UseNodesDataResult {
  canRead: boolean;
  canWrite: boolean;
  nodes: Node[];
  filteredNodes: Node[];
  selectedNode: NodeDetail | null;
  selectedNodePods: Pod[];
  selectedNodeEvents: K8sEvent[];
  lastDrainPreview: NodeDrainPreview | null;
  nodeRuleAlerts: NodeRuleAlert[];
  isDispatchingNodeAlert: boolean;
  isUpdatingNodeAlertLifecycle: boolean;
  selectedNodeNames: string[];
  search: string;
  isLoading: boolean;
  isBusy: boolean;
  error: string | null;
  notice: string | null;
  setSearch: (value: string) => void;
  load: () => Promise<void>;
  openDetail: (name: string) => Promise<void>;
  cordon: (name: string) => Promise<void>;
  uncordon: (name: string) => Promise<void>;
  previewDrain: (name: string) => Promise<void>;
  drain: (name: string, options?: NodeDrainOptions) => Promise<void>;
  toggleNodeSelection: (name: string) => void;
  toggleSelectAllVisible: (names: string[]) => void;
  clearNodeSelection: () => void;
  bulkCordon: () => Promise<void>;
  bulkUncordon: () => Promise<void>;
  bulkDrain: (options?: NodeDrainOptions) => Promise<void>;
  dispatchNodeRuleAlert: (alertID: string) => Promise<void>;
  updateNodeAlertLifecycle: (
    alertID: string,
    status: "acknowledged" | "snoozed" | "dismissed" | "active",
  ) => Promise<void>;
  clearSelectedNode: () => void;
}

/**
 * Compatibility facade for the Nodes view.
 *
 * Data loading, node actions, and bulk selection/actions live in smaller hooks;
 * this facade keeps the public shape stable for the view and existing tests.
 *
 * @returns Nodes state and command handlers for rendering and interaction.
 */
export function useNodesData(): UseNodesDataResult {
  const [isBusy, setIsBusy] = useState(false);
  const list = useNodeList();
  const actions = useNodeActions({
    canRead: list.canRead,
    canWrite: list.canWrite,
    load: list.load,
    reportError: list.reportError,
    reportNotice: list.reportNotice,
    setIsBusy,
  });
  const bulk = useNodeBulkActions({
    canWrite: list.canWrite,
    nodes: list.nodes,
    load: list.load,
    reportError: list.reportError,
    reportNotice: list.reportNotice,
    setIsBusy,
  });

  const nodeRuleAlerts = useMemo(
    () => deriveNodeRuleAlerts(list.nodes, list.clusterEvents, actions.allocatableDropAlerts, list.alertLifecycleByID),
    [actions.allocatableDropAlerts, list.alertLifecycleByID, list.clusterEvents, list.nodes],
  );
  const { isDispatchingNodeAlert, isUpdatingNodeAlertLifecycle, dispatchNodeRuleAlert, updateNodeAlertLifecycle } =
    useNodeAlertActions({
      canWrite: list.canWrite,
      nodeRuleAlerts,
      reportError: list.reportError,
      reportNotice: list.reportNotice,
      setAlertLifecycleByID: list.setAlertLifecycleByID,
    });

  return {
    canRead: list.canRead,
    canWrite: list.canWrite,
    nodes: list.nodes,
    filteredNodes: list.filteredNodes,
    selectedNode: actions.selectedNode,
    selectedNodePods: actions.selectedNodePods,
    selectedNodeEvents: actions.selectedNodeEvents,
    lastDrainPreview: actions.lastDrainPreview,
    nodeRuleAlerts,
    isDispatchingNodeAlert,
    isUpdatingNodeAlertLifecycle,
    selectedNodeNames: bulk.selectedNodeNames,
    search: list.search,
    isLoading: list.isLoading,
    isBusy,
    error: list.error,
    notice: list.notice,
    setSearch: list.setSearch,
    load: list.load,
    openDetail: actions.openDetail,
    cordon: actions.cordon,
    uncordon: actions.uncordon,
    previewDrain: actions.previewDrain,
    drain: actions.drain,
    toggleNodeSelection: bulk.toggleNodeSelection,
    toggleSelectAllVisible: bulk.toggleSelectAllVisible,
    clearNodeSelection: bulk.clearNodeSelection,
    bulkCordon: bulk.bulkCordon,
    bulkUncordon: bulk.bulkUncordon,
    bulkDrain: bulk.bulkDrain,
    dispatchNodeRuleAlert,
    updateNodeAlertLifecycle,
    clearSelectedNode: actions.clearSelectedNode,
  };
}
