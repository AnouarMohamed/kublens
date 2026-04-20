import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AssistantMessage } from "../types";
import { CHAT_SESSIONS_STORAGE_KEY, useChatSessions } from "./useChatSessions";

const baseMessages = (content: string, timestamp = "2026-04-20T00:00:00Z"): AssistantMessage[] => [
  {
    id: `user-${content}`,
    role: "user",
    content,
    timestamp,
  },
];

describe("useChatSessions", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.restoreAllMocks();
  });

  it("auto-selects the most recent stored session", async () => {
    window.localStorage.setItem(
      CHAT_SESSIONS_STORAGE_KEY,
      JSON.stringify([
        { id: "older", title: "Older", startedAt: "2026-04-18T00:00:00Z", messages: baseMessages("older") },
        { id: "newer", title: "Newer", startedAt: "2026-04-19T00:00:00Z", messages: baseMessages("newer") },
      ]),
    );

    const { result } = renderHook(() => useChatSessions());

    await waitFor(() => {
      expect(result.current.activeSessionId).toBe("newer");
      expect(result.current.sessions[0]?.id).toBe("newer");
    });
  });

  it("saves sessions using the first user message as title", async () => {
    const randomUUID = vi.fn().mockReturnValue("session-1");
    Object.defineProperty(globalThis.crypto, "randomUUID", { value: randomUUID, configurable: true });

    const { result } = renderHook(() => useChatSessions());

    let id = "";
    act(() => {
      id = result.current.startNewSession();
    });

    act(() => {
      result.current.saveSession(
        id,
        baseMessages("This is the first user message that should be trimmed at sixty characters exactly."),
      );
    });

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(1);
      expect(result.current.sessions[0]?.title).toBe("This is the first user message that should be trimmed at six");
      expect(window.localStorage.getItem(CHAT_SESSIONS_STORAGE_KEY)).toContain('"id":"session-1"');
    });
  });

  it("prunes to the ten most recent sessions and deletes selected sessions", async () => {
    const { result } = renderHook(() => useChatSessions());

    for (let index = 0; index < 11; index += 1) {
      act(() => {
        result.current.saveSession(
          `session-${index}`,
          baseMessages(`message-${index}`, `2026-04-${String(index + 1).padStart(2, "0")}T00:00:00Z`),
        );
      });
    }

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(10);
      expect(result.current.sessions.some((session) => session.id === "session-0")).toBe(false);
    });

    act(() => {
      result.current.deleteSession("session-10");
    });

    await waitFor(() => {
      expect(result.current.sessions.some((session) => session.id === "session-10")).toBe(false);
      expect(result.current.activeSessionId).toBe("session-9");
    });
  });
});
