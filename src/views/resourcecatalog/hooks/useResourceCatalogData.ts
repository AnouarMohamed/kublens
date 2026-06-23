import { useCallback, useMemo, useState } from "react";
import { useAsyncResource } from "../../../app/hooks/useAsyncResource";
import { getViewItem } from "../../../features/viewCatalog";
import { useAuthSession } from "../../../context/AuthSessionContext";
import { api } from "../../../lib/api";
import type { ResourceRecord, View } from "../../../types";
import { ROLLBACK_VIEWS, RESTARTABLE_VIEWS, SCALEABLE_VIEWS } from "../constants";
import { extractReplicas } from "../utils";

/**
 * State and actions for the generic resource catalog view.
 */
interface UseResourceCatalogDataResult {
  meta: ReturnType<typeof getViewItem>;
  canRead: boolean;
  canWrite: boolean;
  resources: ResourceRecord[];
  search: string;
  isLoading: boolean;
  isActing: boolean;
  error: string | null;
  message: string | null;
  yamlTarget: ResourceRecord | null;
  yamlText: string;
  scaleTarget: ResourceRecord | null;
  scaleReplicas: string;
  filtered: ResourceRecord[];
  hasWorkloadActions: boolean;
  isScaleableView: boolean;
  isRestartableView: boolean;
  isRollbackView: boolean;
  setSearch: (value: string) => void;
  setYAMLTarget: (value: ResourceRecord | null) => void;
  setYAMLText: (value: string) => void;
  setScaleTarget: (value: ResourceRecord | null) => void;
  setScaleReplicas: (value: string) => void;
  load: () => Promise<void>;
  openYAMLEditor: (resource: ResourceRecord) => Promise<void>;
  applyYAML: () => Promise<void>;
  openScaleEditor: (resource: ResourceRecord) => void;
  applyScale: () => Promise<void>;
  restartResource: (resource: ResourceRecord) => Promise<void>;
  rollbackResource: (resource: ResourceRecord) => Promise<void>;
}

/**
 * Manages resource catalog data and workload actions for a given view.
 *
 * @param view - Current catalog view key.
 * @returns Catalog state and action handlers.
 */
export function useResourceCatalogData(view: View): UseResourceCatalogDataResult {
  const { can, isLoading: authLoading } = useAuthSession();
  const meta = getViewItem(view);
  const [search, setSearchState] = useState("");
  const [isActing, setIsActing] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const canRead = can("read");
  const canWrite = can("write");

  const [yamlTarget, setYAMLTargetState] = useState<ResourceRecord | null>(null);
  const [yamlText, setYAMLTextState] = useState("");

  const [scaleTarget, setScaleTargetState] = useState<ResourceRecord | null>(null);
  const [scaleReplicas, setScaleReplicasState] = useState("1");

  const setSearch = useCallback((value: string) => {
    setSearchState(value);
  }, []);

  const setYAMLTarget = useCallback((value: ResourceRecord | null) => {
    setYAMLTargetState(value);
  }, []);

  const setYAMLText = useCallback((value: string) => {
    setYAMLTextState(value);
  }, []);

  const setScaleTarget = useCallback((value: ResourceRecord | null) => {
    setScaleTargetState(value);
  }, []);

  const setScaleReplicas = useCallback((value: string) => {
    setScaleReplicasState(value);
  }, []);

  const loadResources = useCallback(
    async (signal: AbortSignal) => {
      const response = await api.getResources(view, signal);
      return response.items;
    },
    [view],
  );

  const {
    data: resources,
    isLoading,
    error: loadError,
    load: loadCatalogResources,
  } = useAsyncResource<ResourceRecord[]>({
    loader: loadResources,
    fallbackError: "Failed to load resources",
    initialData: [],
    enabled: !authLoading && canRead,
    disabledData: [],
    disabledError: authLoading ? null : "Authenticate to view resource data.",
  });

  const load = useCallback(async () => {
    setActionError(null);
    await loadCatalogResources();
  }, [loadCatalogResources]);

  const error = actionError ?? loadError;

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (query === "") {
      return resources;
    }

    return resources.filter((resource) => {
      const haystack =
        `${resource.name} ${resource.namespace ?? ""} ${resource.status} ${resource.summary ?? ""}`.toLowerCase();
      return haystack.includes(query);
    });
  }, [resources, search]);

  const isScaleableView = SCALEABLE_VIEWS.has(view);
  const isRestartableView = RESTARTABLE_VIEWS.has(view);
  const isRollbackView = ROLLBACK_VIEWS.has(view);
  const hasWorkloadActions = isScaleableView || isRestartableView || isRollbackView;

  const openYAMLEditor = useCallback(
    async (resource: ResourceRecord) => {
      if (!canWrite) {
        setActionError("Your role does not allow YAML actions.");
        return;
      }
      if (!resource.namespace) {
        setActionError("YAML actions require a namespaced resource");
        return;
      }

      setIsActing(true);
      try {
        const response = await api.getResourceYAML(view, resource.namespace, resource.name);
        setYAMLTargetState(resource);
        setYAMLTextState(response.yaml);
        setActionError(null);
      } catch (err) {
        setActionError(err instanceof Error ? err.message : "Failed to load YAML");
      } finally {
        setIsActing(false);
      }
    },
    [canWrite, view],
  );

  const applyYAML = useCallback(async () => {
    if (!canWrite) {
      setActionError("Your role does not allow YAML actions.");
      return;
    }
    if (!yamlTarget || !yamlTarget.namespace) {
      return;
    }

    setIsActing(true);
    try {
      const response = await api.applyResourceYAML(view, yamlTarget.namespace, yamlTarget.name, { yaml: yamlText });
      let finalMessage = response.message;
      if ("requiresForce" in response && response.requiresForce) {
        const force = window.confirm(
          `${response.message}\n\nRisk score: ${response.report.score} (${response.report.level}).\nApply anyway with force=true?`,
        );
        if (!force) {
          setMessage(`Apply canceled. Risk score ${response.report.score} requires explicit force override.`);
          setActionError(null);
          return;
        }

        const forced = await api.applyResourceYAMLWithForce(
          view,
          yamlTarget.namespace,
          yamlTarget.name,
          { yaml: yamlText },
          true,
        );
        if ("requiresForce" in forced && forced.requiresForce) {
          setActionError("Risk guard still blocked the apply request.");
          return;
        }
        finalMessage = forced.message;
      }

      setMessage(finalMessage);
      setYAMLTargetState(null);
      setYAMLTextState("");
      setActionError(null);
      await load();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to apply YAML");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, load, view, yamlTarget, yamlText]);

  const openScaleEditor = useCallback(
    (resource: ResourceRecord) => {
      if (!canWrite) {
        setActionError("Your role does not allow scaling actions.");
        return;
      }
      if (!resource.namespace) {
        return;
      }

      setScaleTargetState(resource);
      setScaleReplicasState(String(extractReplicas(resource.status)));
    },
    [canWrite],
  );

  const applyScale = useCallback(async () => {
    if (!canWrite) {
      setActionError("Your role does not allow scaling actions.");
      return;
    }
    if (!scaleTarget || !scaleTarget.namespace) {
      return;
    }

    const replicas = Number.parseInt(scaleReplicas, 10);
    if (!Number.isFinite(replicas) || replicas < 0) {
      setActionError("Replicas must be a positive integer or zero");
      return;
    }

    setIsActing(true);
    try {
      const response = await api.scaleResource(view, scaleTarget.namespace, scaleTarget.name, { replicas });
      setMessage(response.message);
      setScaleTargetState(null);
      setActionError(null);
      await load();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to scale resource");
    } finally {
      setIsActing(false);
    }
  }, [canWrite, load, scaleReplicas, scaleTarget, view]);

  const restartResource = useCallback(
    async (resource: ResourceRecord) => {
      if (!canWrite) {
        setActionError("Your role does not allow restart actions.");
        return;
      }
      if (!resource.namespace) {
        return;
      }
      if (!window.confirm(`Restart ${view.slice(0, -1)} ${resource.namespace}/${resource.name}?`)) {
        return;
      }

      setIsActing(true);
      try {
        const response = await api.restartResource(view, resource.namespace, resource.name);
        setMessage(response.message);
        setActionError(null);
        await load();
      } catch (err) {
        setActionError(err instanceof Error ? err.message : "Failed to restart resource");
      } finally {
        setIsActing(false);
      }
    },
    [canWrite, load, view],
  );

  const rollbackResource = useCallback(
    async (resource: ResourceRecord) => {
      if (!canWrite) {
        setActionError("Your role does not allow rollback actions.");
        return;
      }
      if (!resource.namespace) {
        return;
      }
      if (!window.confirm(`Rollback deployment ${resource.namespace}/${resource.name}?`)) {
        return;
      }

      setIsActing(true);
      try {
        const response = await api.rollbackResource(view, resource.namespace, resource.name);
        setMessage(response.message);
        setActionError(null);
        await load();
      } catch (err) {
        setActionError(err instanceof Error ? err.message : "Failed to rollback resource");
      } finally {
        setIsActing(false);
      }
    },
    [canWrite, load, view],
  );

  return {
    meta,
    canRead,
    canWrite,
    resources,
    search,
    isLoading,
    isActing,
    error,
    message,
    yamlTarget,
    yamlText,
    scaleTarget,
    scaleReplicas,
    filtered,
    hasWorkloadActions,
    isScaleableView,
    isRestartableView,
    isRollbackView,
    setSearch,
    setYAMLTarget,
    setYAMLText,
    setScaleTarget,
    setScaleReplicas,
    load,
    openYAMLEditor,
    applyYAML,
    openScaleEditor,
    applyScale,
    restartResource,
    rollbackResource,
  };
}
