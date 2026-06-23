import { useCallback, useEffect, useRef, useState, type SetStateAction } from "react";
import { toErrorMessage } from "./asyncTask";

interface UseAsyncResourceOptions<T> {
  loader: (signal: AbortSignal) => Promise<T>;
  fallbackError: string;
  initialData: T;
  autoLoad?: boolean;
  enabled?: boolean;
  disabledData?: T;
  disabledError?: string | null;
  refreshMs?: number;
  refreshEnabled?: boolean;
}

interface UseAsyncResourceResult<T> {
  data: T;
  isLoading: boolean;
  error: string | null;
  load: (backgroundRefresh?: boolean) => Promise<void>;
  updateData: (nextData: SetStateAction<T>) => void;
}

/**
 * Shared async resource loader for read-heavy views.
 *
 * It owns loading/error state, aborts superseded requests, and ignores stale
 * responses so views can focus on data shaping instead of request lifecycle.
 * Action-heavy views can use updateData for local mutation results while still
 * keeping initial reads, denied states, and refreshes centralized here.
 */
export function useAsyncResource<T>({
  loader,
  fallbackError,
  initialData,
  autoLoad = true,
  enabled = true,
  disabledData,
  disabledError = null,
  refreshMs,
  refreshEnabled = true,
}: UseAsyncResourceOptions<T>): UseAsyncResourceResult<T> {
  const [data, setData] = useState<T>(initialData);
  const [isLoading, setIsLoading] = useState(autoLoad);
  const [error, setError] = useState<string | null>(null);
  const initialDataRef = useRef(initialData);
  const disabledDataRef = useRef(disabledData);
  const requestSeqRef = useRef(0);
  const activeControllerRef = useRef<AbortController | null>(null);

  initialDataRef.current = initialData;
  disabledDataRef.current = disabledData;

  const load = useCallback(
    async (backgroundRefresh = false) => {
      if (!enabled) {
        activeControllerRef.current?.abort();
        activeControllerRef.current = null;
        requestSeqRef.current += 1;
        setData(disabledDataRef.current ?? initialDataRef.current);
        setError(disabledError);
        setIsLoading(false);
        return;
      }

      const requestID = requestSeqRef.current + 1;
      requestSeqRef.current = requestID;

      activeControllerRef.current?.abort();
      const controller = new AbortController();
      activeControllerRef.current = controller;

      if (!backgroundRefresh) {
        setIsLoading(true);
      }

      try {
        const payload = await loader(controller.signal);
        if (controller.signal.aborted || requestID !== requestSeqRef.current) {
          return;
        }
        setData(payload);
        setError(null);
      } catch (err) {
        if (controller.signal.aborted || requestID !== requestSeqRef.current) {
          return;
        }
        setError(toErrorMessage(err, fallbackError));
      } finally {
        if (activeControllerRef.current === controller) {
          activeControllerRef.current = null;
        }
        if (!backgroundRefresh && requestID === requestSeqRef.current) {
          setIsLoading(false);
        }
      }
    },
    [disabledError, enabled, fallbackError, loader],
  );

  const updateData = useCallback((nextData: SetStateAction<T>) => {
    setData(nextData);
  }, []);

  useEffect(() => {
    if (!autoLoad) {
      return undefined;
    }
    void load(false);
    return undefined;
  }, [autoLoad, load]);

  useEffect(() => {
    if (!refreshEnabled || !refreshMs || refreshMs <= 0) {
      return undefined;
    }

    const handle = window.setInterval(() => {
      void load(true);
    }, refreshMs);

    return () => window.clearInterval(handle);
  }, [load, refreshEnabled, refreshMs]);

  useEffect(() => {
    return () => {
      requestSeqRef.current += 1;
      activeControllerRef.current?.abort();
      activeControllerRef.current = null;
    };
  }, []);

  return { data, isLoading, error, load, updateData };
}
