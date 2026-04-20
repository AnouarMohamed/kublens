/**
 * Maintains the currently selected application view with local persistence.
 *
 * The hook resolves an initial view from the URL path first, then falls back
 * to `localStorage` when no explicit deep-link path is provided.
 */
import { useEffect, useState } from "react";
import type { View } from "../../types";

const VIEW_KEY = "k8s-ops.current-view.v1";

const VALID_VIEWS = new Set<View>([
  "overview",
  "pods",
  "deployments",
  "replicasets",
  "statefulsets",
  "daemonsets",
  "jobs",
  "cronjobs",
  "services",
  "ingresses",
  "networkpolicies",
  "configmaps",
  "secrets",
  "persistentvolumes",
  "persistentvolumeclaims",
  "storageclasses",
  "nodes",
  "namespaces",
  "events",
  "serviceaccounts",
  "rbac",
  "metrics",
  "slo",
  "rightsizing",
  "audit",
  "predictions",
  "diagnostics",
  "assistant",
  "incidents",
  "remediation",
  "memory",
  "shiftbrief",
  "playbooks",
  "riskguard",
  "postmortems",
]);

function loadLastView(): View {
  try {
    const pathname = window.location.pathname.toLowerCase();
    const pathView = mapPathToView(pathname);
    if (pathView) {
      return pathView;
    }

    const raw = window.localStorage.getItem(VIEW_KEY);
    if (raw && VALID_VIEWS.has(raw as View)) {
      return raw as View;
    }
  } catch {
    // no-op
  }
  return "overview";
}

/**
 * useCurrentView exposes the active view and setter used by the main shell.
 *
 * @returns Current view state and setter.
 */
export function useCurrentView() {
  const [currentView, setCurrentView] = useState<View>(loadLastView);

  useEffect(() => {
    window.localStorage.setItem(VIEW_KEY, currentView);
  }, [currentView]);

  return { currentView, setCurrentView };
}

/**
 * Maps deep-link URL paths to view identifiers.
 *
 * @param pathname - Browser pathname in lowercase.
 * @returns Matching view or `null` when no mapping exists.
 */
function mapPathToView(pathname: string): View | null {
  const mapping: Array<{ prefix: string; view: View }> = [
    { prefix: "/incidents", view: "incidents" },
    { prefix: "/remediation", view: "remediation" },
    { prefix: "/memory", view: "memory" },
    { prefix: "/shift-brief", view: "shiftbrief" },
    { prefix: "/shiftbrief", view: "shiftbrief" },
    { prefix: "/playbooks", view: "playbooks" },
    { prefix: "/risk-guard", view: "riskguard" },
    { prefix: "/riskguard", view: "riskguard" },
    { prefix: "/postmortems", view: "postmortems" },
    { prefix: "/slo", view: "slo" },
    { prefix: "/rightsizing", view: "rightsizing" },
    { prefix: "/assistant", view: "assistant" },
  ];
  for (const item of mapping) {
    if (pathname === item.prefix || pathname.startsWith(item.prefix + "/")) {
      return item.view;
    }
  }
  return null;
}
