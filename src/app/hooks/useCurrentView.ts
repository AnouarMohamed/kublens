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
  "workbench",
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
  "ghost",
  "assistant",
  "incidents",
  "remediation",
  "memory",
  "shiftbrief",
  "playbooks",
  "riskguard",
  "postmortems",
]);

const CANONICAL_VIEW_PATHS: Partial<Record<View, string>> = {
  workbench: "/",
  overview: "/overview",
  shiftbrief: "/shift-brief",
  riskguard: "/risk-guard",
};

function canonicalPathForView(view: View): string {
  return CANONICAL_VIEW_PATHS[view] ?? `/${view}`;
}

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
    try {
      const pathname = window.location.pathname.toLowerCase();
      const targetPath = canonicalPathForView(currentView);
      if (pathname !== targetPath) {
        window.history.pushState({ view: currentView }, "", targetPath);
      }
    } catch {
      // Safely ignore history API access errors in test environment
    }
  }, [currentView]);

  useEffect(() => {
    const handlePopState = (event: PopStateEvent) => {
      const state = event.state as { view?: View } | null;
      if (state?.view && VALID_VIEWS.has(state.view)) {
        setCurrentView(state.view);
      } else {
        try {
          const pathView = mapPathToView(window.location.pathname.toLowerCase());
          if (pathView) {
            setCurrentView(pathView);
          }
        } catch {
          // ignore
        }
      }
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  return { currentView, setCurrentView };
}

/**
 * Maps deep-link URL paths to view identifiers.
 *
 * @param pathname - Browser pathname in lowercase.
 * @returns Matching view or `null` when no mapping exists.
 */
function mapPathToView(pathname: string): View | null {
  const cleanPath = pathname.replace(/^\/+/, "").split("/")[0];
  if (cleanPath === "") {
    return "workbench";
  }
  if (cleanPath === "shift-brief") {
    return "shiftbrief";
  }
  if (cleanPath === "risk-guard") {
    return "riskguard";
  }
  if (VALID_VIEWS.has(cleanPath as View)) {
    return cleanPath as View;
  }
  return null;
}
