/**
 * Main application shell that composes navigation, view routing, and utility panels.
 */
import { Suspense, lazy, useEffect, useMemo, useRef, useState, type ReactElement } from "react";
import Sidebar from "../components/Sidebar";
import { getViewItem } from "../features/viewCatalog";
import { useAuthSession } from "../context/AuthSessionContext";
import { CommandPalette } from "./components/CommandPalette";
import { HeaderBar } from "./components/HeaderBar";
import { WorkspacePanels } from "./components/WorkspacePanels";
import { useCurrentView } from "./hooks/useCurrentView";
import { useNotifications } from "./hooks/useNotifications";
import { useUserSettings } from "./hooks/useUserSettings";
import { useClusterContexts } from "./hooks/useClusterContexts";
import { useRuntimeStatus } from "./hooks/useRuntimeStatus";
import { useClusterSwitcher } from "./hooks/useClusterSwitcher";
import { useSearchNavigation } from "./hooks/useSearchNavigation";
import { blockedViewMessage, useViewAccess } from "./hooks/useViewAccess";
import { useTransientMessage } from "./hooks/useTransientMessage";
import { CLUSTER_REFRESH_EVENT, VIEW_NAVIGATE_EVENT, type ViewNavigateDetail } from "./viewNavigation";
import type { View } from "../types";
import Dashboard from "../views/dashboard";

const Metrics = lazy(() => import("../views/metrics"));
const SLOCenter = lazy(() => import("../views/slo"));
const Rightsizing = lazy(() => import("../views/rightsizing"));
const Audit = lazy(() => import("../views/audit"));
const Pods = lazy(() => import("../views/pods"));
const Deployments = lazy(() => import("../views/deployments"));
const Nodes = lazy(() => import("../views/nodes"));
const Events = lazy(() => import("../views/events"));
const Namespaces = lazy(() => import("../views/namespaces"));
const RBAC = lazy(() => import("../views/rbac"));
const Diagnostics = lazy(() => import("../views/diagnostics"));
const Predictions = lazy(() => import("../views/predictions"));
const OpsAssistant = lazy(() => import("../views/opsassistant"));
const IncidentCommander = lazy(() => import("../views/incident"));
const Remediation = lazy(() => import("../views/remediation"));
const ClusterMemory = lazy(() => import("../views/memory"));
const ShiftBrief = lazy(() => import("../views/shiftbrief"));
const Playbooks = lazy(() => import("../views/playbooks"));
const RiskGuard = lazy(() => import("../views/riskguard"));
const Postmortems = lazy(() => import("../views/postmortem"));
const ResourceCatalog = lazy(() => import("../views/resourcecatalog"));

type Panel = "none" | "notifications" | "settings" | "profile";

export function AppShell() {
  const {
    session: authSession,
    isLoading: authLoading,
    lastRefreshAt,
    lastLoginAt,
    lastLogoutAt,
    failedLoginCount,
    can,
    login,
    logout,
    refresh: refreshSession,
  } = useAuthSession();
  const { currentView, setCurrentView } = useCurrentView();
  const { settings, setSettings, resetSettings } = useUserSettings();
  const [panel, setPanel] = useState<Panel>("none");
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [authToken, setAuthToken] = useState("");
  const [authMessage, setAuthMessage] = useState<string | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const inactivityTimerRef = useRef<number | null>(null);
  const inactivityLogoutInFlightRef = useRef(false);

  const canRead = can("read");
  const runtime = useRuntimeStatus({ authLoading, canRead });
  const { clusterContexts, setClusterContexts } = useClusterContexts({ authLoading, canRead });
  const {
    notifications,
    notificationError,
    notificationStatus,
    notificationLastUpdatedAt,
    notificationUnreadCount,
    notificationSuppressedCount,
    notificationSignal,
    markNotificationsRead,
    clearNotifications,
  } = useNotifications({
    panel,
    authLoading,
    canRead,
    canStream: can("stream"),
    autoRefreshSeconds: settings.autoRefreshSeconds,
    notificationLimit: settings.notificationLimit,
    notificationBurstThreshold: settings.notificationBurstThreshold,
    liveNotificationsEnabled: settings.liveNotifications,
    desktopNotificationsEnabled: settings.desktopNotifications,
    mutedKeywords: settings.mutedNotificationKeywords,
    redactSensitiveNotifications: settings.redactSensitiveNotifications,
  });

  const { message: transientMessage, showMessage } = useTransientMessage();
  const { sections, searchableItems, isAllowed } = useViewAccess({
    canAssist: can("assist"),
  });
  const { search, setSearch, submitSearch } = useSearchNavigation({
    items: searchableItems,
    setCurrentView,
    onMessage: (message) => showMessage(message, 1500),
  });
  const { clusterRefreshKey, isSwitchingCluster, refreshCluster, selectCluster } = useClusterSwitcher({
    clusterContexts,
    setClusterContexts,
    onMessage: (message) => showMessage(message, 1800),
  });

  const currentViewMeta = getViewItem(currentView);
  const renderedView = useMemo(() => renderView(currentView), [currentView]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen(true);
        setPanel("none");
        return;
      }
      if (event.key === "/" && document.activeElement !== searchRef.current) {
        event.preventDefault();
        searchRef.current?.focus();
        return;
      }
      if (event.key === "Escape") {
        setPanel("none");
        setPaletteOpen(false);
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    if (authLoading) {
      return;
    }
    if (!isAllowed(currentView)) {
      setCurrentView("overview");
      showMessage(blockedViewMessage(currentView), 1800);
    }
  }, [authLoading, currentView, isAllowed, setCurrentView, showMessage]);

  useEffect(() => {
    const onNavigate = (event: Event) => {
      const custom = event as CustomEvent<ViewNavigateDetail>;
      const targetView = custom.detail?.view;
      if (!targetView) {
        return;
      }
      if (!isAllowed(targetView)) {
        showMessage(blockedViewMessage(targetView), 1800);
        return;
      }
      setCurrentView(targetView);
      setPanel("none");
      setPaletteOpen(false);
    };

    window.addEventListener(VIEW_NAVIGATE_EVENT, onNavigate as EventListener);
    return () => window.removeEventListener(VIEW_NAVIGATE_EVENT, onNavigate as EventListener);
  }, [isAllowed, setCurrentView, showMessage]);

  useEffect(() => {
    const onRefresh = () => {
      refreshCluster();
      setPanel("none");
      setPaletteOpen(false);
      showMessage("Cluster view refreshed.", 1500);
    };

    window.addEventListener(CLUSTER_REFRESH_EVENT, onRefresh);
    return () => window.removeEventListener(CLUSTER_REFRESH_EVENT, onRefresh);
  }, [refreshCluster, showMessage]);

  useEffect(() => {
    if (inactivityTimerRef.current !== null) {
      window.clearTimeout(inactivityTimerRef.current);
      inactivityTimerRef.current = null;
    }

    const timeoutMinutes = settings.inactivityLogoutMinutes;
    if (authLoading || timeoutMinutes <= 0 || !authSession?.enabled || !authSession.authenticated) {
      return;
    }

    const timeoutMs = timeoutMinutes * 60 * 1000;
    const onTimeout = () => {
      if (inactivityLogoutInFlightRef.current) {
        return;
      }
      inactivityLogoutInFlightRef.current = true;
      void logout()
        .catch(() => {
          // Best effort: session may already be invalid server-side.
        })
        .finally(() => {
          void refreshSession().finally(() => {
            inactivityLogoutInFlightRef.current = false;
            showMessage(`Session auto-logged out after ${timeoutMinutes}m of inactivity.`, 2600);
          });
        });
    };

    const resetTimer = () => {
      if (inactivityTimerRef.current !== null) {
        window.clearTimeout(inactivityTimerRef.current);
      }
      inactivityTimerRef.current = window.setTimeout(onTimeout, timeoutMs);
    };

    const activityEvents: Array<keyof WindowEventMap> = ["mousedown", "keydown", "touchstart", "scroll"];
    for (const eventName of activityEvents) {
      window.addEventListener(eventName, resetTimer, { passive: true });
    }
    resetTimer();

    return () => {
      if (inactivityTimerRef.current !== null) {
        window.clearTimeout(inactivityTimerRef.current);
        inactivityTimerRef.current = null;
      }
      for (const eventName of activityEvents) {
        window.removeEventListener(eventName, resetTimer);
      }
    };
  }, [
    authLoading,
    authSession?.authenticated,
    authSession?.enabled,
    logout,
    refreshSession,
    settings.inactivityLogoutMinutes,
    showMessage,
  ]);

  return (
    <div className={`flex h-screen text-zinc-100 ${settings.denseMode ? "text-[13px]" : "text-sm"}`}>
      <Sidebar
        key={`sidebar-${clusterRefreshKey}`}
        currentView={currentView}
        onViewChange={setCurrentView}
        sections={sections}
      />

      <main className="flex-1 flex flex-col overflow-hidden p-4 pl-0">
        <div className="app-shell relative flex-1 flex flex-col overflow-hidden">
          <HeaderBar
            currentViewMeta={currentViewMeta}
            clusterContexts={clusterContexts}
            runtime={runtime}
            isSwitchingCluster={isSwitchingCluster}
            search={search}
            onSearchChange={setSearch}
            onSearchSubmit={submitSearch}
            onSelectCluster={selectCluster}
            onToggleNotifications={() => setPanel((value) => (value === "notifications" ? "none" : "notifications"))}
            onToggleSettings={() => setPanel((value) => (value === "settings" ? "none" : "settings"))}
            onToggleProfile={() => setPanel((value) => (value === "profile" ? "none" : "profile"))}
            notificationStatus={notificationStatus}
            notificationUnreadCount={notificationUnreadCount}
            searchRef={searchRef}
          />

          {transientMessage && (
            <div className="px-6 py-2 bg-[var(--accent-dim)] text-zinc-100 text-xs tracking-wide border-b border-zinc-700">
              {transientMessage}
            </div>
          )}

          <div key={`view-${clusterRefreshKey}`} className="flex-1 overflow-y-auto p-6 bg-grid">
            {renderedView}
          </div>

          <WorkspacePanels
            panel={panel}
            notifications={notifications}
            notificationError={notificationError}
            settings={settings}
            setSettings={setSettings}
            resetSettings={resetSettings}
            runtime={runtime}
            authSession={authSession}
            authLoading={authLoading}
            authToken={authToken}
            setAuthToken={setAuthToken}
            authMessage={authMessage}
            onAuthMessage={setAuthMessage}
            login={login}
            logout={logout}
            refreshSession={refreshSession}
            authLastRefreshAt={lastRefreshAt}
            authLastLoginAt={lastLoginAt}
            authLastLogoutAt={lastLogoutAt}
            authFailedLoginCount={failedLoginCount}
            currentCommand={currentViewMeta.kubectlCommand}
            notificationStatus={notificationStatus}
            notificationLastUpdatedAt={notificationLastUpdatedAt}
            notificationUnreadCount={notificationUnreadCount}
            notificationSuppressedCount={notificationSuppressedCount}
            notificationSignal={notificationSignal}
            markNotificationsRead={markNotificationsRead}
            clearNotifications={clearNotifications}
            openEventsView={() => {
              setCurrentView("events");
              setPanel("none");
            }}
          />

          {paletteOpen && (
            <CommandPalette
              paletteOpen={paletteOpen}
              setPaletteOpen={setPaletteOpen}
              sections={sections}
              searchableItems={searchableItems}
            />
          )}
        </div>
      </main>
    </div>
  );
}

function renderView(view: View): ReactElement {
  switch (view) {
    case "overview":
      return <Dashboard />;
    case "pods":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading pods..." />}>
          <Pods />
        </Suspense>
      );
    case "deployments":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading deployments..." />}>
          <Deployments />
        </Suspense>
      );
    case "nodes":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading nodes..." />}>
          <Nodes />
        </Suspense>
      );
    case "events":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading events..." />}>
          <Events />
        </Suspense>
      );
    case "namespaces":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading namespaces..." />}>
          <Namespaces />
        </Suspense>
      );
    case "rbac":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading RBAC..." />}>
          <RBAC />
        </Suspense>
      );
    case "metrics":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading metrics..." />}>
          <Metrics />
        </Suspense>
      );
    case "slo":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading slo center..." />}>
          <SLOCenter />
        </Suspense>
      );
    case "rightsizing":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading rightsizing advisor..." />}>
          <Rightsizing />
        </Suspense>
      );
    case "audit":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading audit trail..." />}>
          <Audit />
        </Suspense>
      );
    case "predictions":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading predictions..." />}>
          <Predictions />
        </Suspense>
      );
    case "diagnostics":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading diagnostics..." />}>
          <Diagnostics />
        </Suspense>
      );
    case "assistant":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading assistant..." />}>
          <OpsAssistant />
        </Suspense>
      );
    case "incidents":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading incidents..." />}>
          <IncidentCommander />
        </Suspense>
      );
    case "remediation":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading remediation..." />}>
          <Remediation />
        </Suspense>
      );
    case "memory":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading cluster memory..." />}>
          <ClusterMemory />
        </Suspense>
      );
    case "shiftbrief":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading shift brief..." />}>
          <ShiftBrief />
        </Suspense>
      );
    case "playbooks":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading playbooks..." />}>
          <Playbooks />
        </Suspense>
      );
    case "riskguard":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading risk guard..." />}>
          <RiskGuard />
        </Suspense>
      );
    case "postmortems":
      return (
        <Suspense fallback={<ViewLoadingState label="Loading postmortems..." />}>
          <Postmortems />
        </Suspense>
      );
    default:
      return (
        <Suspense fallback={<ViewLoadingState label="Loading resources..." />}>
          <ResourceCatalog view={view} />
        </Suspense>
      );
  }
}

function ViewLoadingState({ label }: { label: string }) {
  return <div className="surface p-6 text-sm text-zinc-300">{label}</div>;
}
