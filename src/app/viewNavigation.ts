import type { View } from "../types";

export const VIEW_NAVIGATE_EVENT = "kubelens:navigate-view";
export const CLUSTER_REFRESH_EVENT = "kubelens:refresh-cluster";

export interface ViewNavigateDetail {
  view: View;
}

/**
 * Requests top-level view navigation from nested components.
 */
export function navigateToView(view: View): void {
  window.dispatchEvent(
    new CustomEvent<ViewNavigateDetail>(VIEW_NAVIGATE_EVENT, {
      detail: { view },
    }),
  );
}

export function requestClusterRefresh(): void {
  window.dispatchEvent(new Event(CLUSTER_REFRESH_EVENT));
}
