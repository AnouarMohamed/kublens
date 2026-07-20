/**
 * Generic WebSocket stream hook for view-level refresh triggers.
 */
import { useEffect } from "react";
import { api } from "../../lib/api";
import { parseStreamEvent } from "../../lib/api/stream";
import type { StreamEvent } from "../../types";

/**
 * Configuration for {@link useStreamRefresh}.
 */
interface UseStreamRefreshOptions<T = unknown> {
  enabled: boolean;
  eventTypes: string[];
  onEvent: (event: StreamEvent<T>) => void;
}

/**
 * Subscribes to stream events and forwards matching payloads to a callback.
 *
 * @typeParam T - Expected event payload type.
 * @param options - Stream activation settings and event callback.
 */
export function useStreamRefresh<T>({ enabled, eventTypes, onEvent }: UseStreamRefreshOptions<T>) {
  useEffect(() => {
    if (!enabled || eventTypes.length === 0) {
      return;
    }

    let cancelled = false;
    const url = typeof api.getStreamWSURL === "function" ? api.getStreamWSURL() : "";
    if (!url) {
      return;
    }

    let socket: WebSocket | null = null;
    try {
      socket = new WebSocket(url);
    } catch {
      return;
    }

    socket.onmessage = (event) => {
      if (cancelled) {
        return;
      }
      const payload = parseStreamEvent<T>(event.data);
      if (!payload) {
        return;
      }
      if (!eventTypes.includes(payload.type)) {
        return;
      }
      onEvent(payload);
    };

    return () => {
      cancelled = true;
      socket?.close();
    };
  }, [enabled, eventTypes, onEvent]);
}
