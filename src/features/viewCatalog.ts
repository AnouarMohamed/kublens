/**
 * View metadata catalog used by navigation, keyboard search, and access policy checks.
 */
import type { View } from "../types";

/**
 * Describes one navigable UI view.
 */
export interface ViewItem {
  id: View;
  label: string;
  description: string;
  kubectlCommand: string;
}

/**
 * Groups related views under a sidebar section.
 */
export interface ViewSection {
  id: string;
  label: string;
  items: ViewItem[];
}

/**
 * Feature gates that affect view visibility.
 */
export interface ViewAccessPolicy {
  assistantEnabled: boolean;
}

/**
 * Static sidebar/view catalog for the application shell.
 */
export const VIEW_SECTIONS: ViewSection[] = [
  {
    id: "overview",
    label: "Overview",
    items: [
      {
        id: "workbench",
        label: "Incident Workbench",
        description: "Detect, simulate, remediate, and audit from one queue.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "overview",
        label: "Cluster Overview",
        description: "Health, capacity, and workload summary in one view.",
        kubectlCommand: "kubectl cluster-info",
      },
    ],
  },
  {
    id: "workloads",
    label: "Workloads",
    items: [
      {
        id: "pods",
        label: "Pods",
        description: "Inspect pod health and actions.",
        kubectlCommand: "kubectl get pods -A",
      },
      {
        id: "deployments",
        label: "Deployments",
        description: "Review rollout state and replica health.",
        kubectlCommand: "kubectl get deployments -A",
      },
      {
        id: "replicasets",
        label: "ReplicaSets",
        description: "Track desired versus ready replicas.",
        kubectlCommand: "kubectl get replicasets -A",
      },
      {
        id: "statefulsets",
        label: "StatefulSets",
        description: "Manage stateful workloads and storage identity.",
        kubectlCommand: "kubectl get statefulsets -A",
      },
      {
        id: "daemonsets",
        label: "DaemonSets",
        description: "Observe node-level agents and rollout health.",
        kubectlCommand: "kubectl get daemonsets -A",
      },
      {
        id: "jobs",
        label: "Jobs",
        description: "Monitor batch runs and completion state.",
        kubectlCommand: "kubectl get jobs -A",
      },
      {
        id: "cronjobs",
        label: "CronJobs",
        description: "Monitor schedules and recurring batch execution.",
        kubectlCommand: "kubectl get cronjobs -A",
      },
    ],
  },
  {
    id: "networking",
    label: "Networking",
    items: [
      {
        id: "services",
        label: "Services",
        description: "Service endpoints and exposure model.",
        kubectlCommand: "kubectl get svc -A",
      },
      {
        id: "ingresses",
        label: "Ingresses",
        description: "Routing rules and external entry points.",
        kubectlCommand: "kubectl get ingress -A",
      },
      {
        id: "networkpolicies",
        label: "Network Policies",
        description: "Traffic isolation policies.",
        kubectlCommand: "kubectl get networkpolicy -A",
      },
    ],
  },
  {
    id: "configuration",
    label: "Configuration",
    items: [
      {
        id: "configmaps",
        label: "ConfigMaps",
        description: "Runtime configuration objects.",
        kubectlCommand: "kubectl get configmaps -A",
      },
      {
        id: "secrets",
        label: "Secrets",
        description: "Secret inventory and usage footprint.",
        kubectlCommand: "kubectl get secrets -A",
      },
    ],
  },
  {
    id: "storage",
    label: "Storage",
    items: [
      {
        id: "persistentvolumes",
        label: "PersistentVolumes",
        description: "Cluster-level storage inventory.",
        kubectlCommand: "kubectl get pv",
      },
      {
        id: "persistentvolumeclaims",
        label: "PersistentVolumeClaims",
        description: "Namespace volume claims.",
        kubectlCommand: "kubectl get pvc -A",
      },
      {
        id: "storageclasses",
        label: "StorageClasses",
        description: "Provisioner and policy definitions.",
        kubectlCommand: "kubectl get storageclass",
      },
    ],
  },
  {
    id: "cluster",
    label: "Cluster",
    items: [
      {
        id: "nodes",
        label: "Nodes",
        description: "Node readiness and resource pressure.",
        kubectlCommand: "kubectl get nodes",
      },
      {
        id: "namespaces",
        label: "Namespaces",
        description: "Namespace boundaries and lifecycle.",
        kubectlCommand: "kubectl get namespaces",
      },
      {
        id: "events",
        label: "Events",
        description: "Recent cluster warnings and changes.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
    ],
  },
  {
    id: "access",
    label: "Access",
    items: [
      {
        id: "serviceaccounts",
        label: "Service Accounts",
        description: "Workload identities and token-linked objects.",
        kubectlCommand: "kubectl get serviceaccounts -A",
      },
      {
        id: "rbac",
        label: "RBAC",
        description: "Roles and bindings overview.",
        kubectlCommand: "kubectl get roles,rolebindings,clusterroles,clusterrolebindings -A",
      },
    ],
  },
  {
    id: "observability",
    label: "Observability",
    items: [
      {
        id: "metrics",
        label: "Metrics",
        description: "Interactive analytics, graphs, trends, and API telemetry.",
        kubectlCommand: "kubectl top pods -A",
      },
      {
        id: "slo",
        label: "SLO Center",
        description: "Error budget posture, burn rate, and incident response guardrails.",
        kubectlCommand: "kubectl get --raw /metrics",
      },
      {
        id: "rightsizing",
        label: "Rightsizing",
        description: "Cost and resource-efficiency recommendations with GitOps-ready patches.",
        kubectlCommand: "kubectl top pods -A",
      },
      {
        id: "audit",
        label: "Audit Trail",
        description: "Live request and action history with operator attribution.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "predictions",
        label: "Predictions",
        description: "Rule-based risk scoring from pod and node health signals.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "diagnostics",
        label: "Diagnostics",
        description: "Automated issue detection and remediation guidance.",
        kubectlCommand: "kubectl describe nodes",
      },
      {
        id: "ghost",
        label: "Ghost Mode",
        description: "Simulate node maintenance before touching the live cluster.",
        kubectlCommand: "kubectl drain <node> --dry-run=server",
      },
    ],
  },
  {
    id: "ai",
    label: "Ops",
    items: [
      {
        id: "shiftbrief",
        label: "Shift Brief",
        description: "On-call handoff snapshot of risk, incidents, and recent changes.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "playbooks",
        label: "Playbooks",
        description: "Structured response guides for recurring production issues.",
        kubectlCommand: "kubectl describe node <name>",
      },
      {
        id: "incidents",
        label: "Incidents",
        description: "Incident commander timeline and runbook execution.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "remediation",
        label: "Remediation",
        description: "Risk-scored remediation proposals with controlled execution.",
        kubectlCommand: "kubectl rollout restart deployment/<name> -n <namespace>",
      },
      {
        id: "memory",
        label: "Cluster Memory",
        description: "Team runbooks and fix patterns for institutional learning.",
        kubectlCommand: "kubectl describe pod <name> -n <namespace>",
      },
      {
        id: "riskguard",
        label: "Risk Guard",
        description: "Manifest risk checks before deployment.",
        kubectlCommand: "kubectl apply --dry-run=server -f manifest.yaml",
      },
      {
        id: "postmortems",
        label: "Postmortems",
        description: "Generated incident postmortems with deterministic timeline.",
        kubectlCommand: "kubectl get events -A --sort-by=.metadata.creationTimestamp",
      },
      {
        id: "assistant",
        label: "Assistant",
        description: "Deterministic plus LLM-assisted troubleshooting.",
        kubectlCommand: "kubectl get pods -A",
      },
    ],
  },
];

export const VIEW_MAP: Record<View, ViewItem> = VIEW_SECTIONS.flatMap((section) => section.items).reduce(
  (acc, item) => {
    acc[item.id] = item;
    return acc;
  },
  {} as Record<View, ViewItem>,
);

/**
 * Returns catalog metadata for a specific view identifier.
 *
 * @param view - View identifier.
 * @returns Matching view metadata.
 */
export function getViewItem(view: View): ViewItem {
  return VIEW_MAP[view];
}

/**
 * Evaluates whether a view is visible under the provided access policy.
 *
 * @param view - Candidate view.
 * @param policy - Access policy.
 * @returns `true` when the view should be shown.
 */
export function isViewVisible(view: View, policy: ViewAccessPolicy): boolean {
  if (view === "assistant") {
    return policy.assistantEnabled;
  }
  return true;
}

/**
 * Filters sidebar sections by visibility policy.
 *
 * @param sections - Full section list.
 * @param policy - Access policy.
 * @returns Sections with hidden views removed.
 */
export function filterSectionsByPolicy(sections: ViewSection[], policy: ViewAccessPolicy): ViewSection[] {
  return sections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => isViewVisible(item.id, policy)),
    }))
    .filter((section) => section.items.length > 0);
}

/**
 * Flattens sectioned views into a single searchable list.
 *
 * @param sections - View sections.
 * @returns Flat view metadata list.
 */
export function flattenViewItems(sections: ViewSection[]): ViewItem[] {
  return sections.flatMap((section) => section.items);
}

/**
 * Finds a view by free-text label or identifier match.
 *
 * @param query - User-entered query.
 * @param items - Optional candidate list.
 * @returns First matching view or `null`.
 */
export function findViewByQuery(query: string, items: ViewItem[] = Object.values(VIEW_MAP)): ViewItem | null {
  const normalized = query.trim().toLowerCase();
  if (normalized === "") {
    return null;
  }

  return (
    items.find((item) => item.label.toLowerCase().includes(normalized) || item.id.toLowerCase().includes(normalized)) ??
    null
  );
}
