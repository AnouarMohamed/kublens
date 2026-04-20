import type { View } from "../types";

export const VIEW_NAVIGATE_EVENT = "kubelens:navigate-view";
export const CLUSTER_REFRESH_EVENT = "kubelens:refresh-cluster";

export interface ViewNavigateDetail {
  view: View;
  prefillMessage?: string;
}

let pendingViewNavigationDetail: ViewNavigateDetail | null = null;

/**
 * Requests top-level view navigation from nested components.
 */
export function navigateToView(view: View, detail: Omit<ViewNavigateDetail, "view"> = {}): void {
  const payload: ViewNavigateDetail = {
    view,
    ...detail,
  };
  pendingViewNavigationDetail = payload;
  window.dispatchEvent(
    new CustomEvent<ViewNavigateDetail>(VIEW_NAVIGATE_EVENT, {
      detail: payload,
    }),
  );
}

export function requestClusterRefresh(): void {
  window.dispatchEvent(new Event(CLUSTER_REFRESH_EVENT));
}

export function consumePendingViewNavigationDetail(): ViewNavigateDetail | null {
  const detail = pendingViewNavigationDetail;
  pendingViewNavigationDetail = null;
  return detail;
}
