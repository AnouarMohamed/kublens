import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../../lib/api";
import type { AssistantResponse } from "../../../types";
import type { AssistantMessage } from "../types";
import { buildDiagnosticPrompts, buildDiagnosticsIntroMessage } from "./assistantChatDiagnostics";
import { useChatSessions } from "./useChatSessions";

export const basePrompts = [
  "Diagnose payment-gateway",
  "Show cluster health",
  "What should I fix first?",
  "Show failed pods",
  "Generate deployment manifest",
] as const;

const initialAssistantMessageTemplate: Omit<AssistantMessage, "id" | "timestamp"> = {
  role: "assistant",
  content: "Assistant is ready. Ask for diagnostics, root causes, and concrete next actions.",
  hints: [...basePrompts],
};

export function useAssistantChat() {
  const { sessions, activeSessionId, startNewSession, selectSession, saveSession, deleteSession } = useChatSessions();
  const [messages, setMessages] = useState<AssistantMessage[]>(() =>
    sessions.length > 0 ? [] : [createAssistantIntroMessage()],
  );
  const [isLoading, setIsLoading] = useState(false);
  const [diagnosticPrompts, setDiagnosticPrompts] = useState<string[]>([]);
  const diagnosticsLoaded = useRef(false);

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId) ?? null,
    [activeSessionId, sessions],
  );

  useEffect(() => {
    if (activeSession) {
      setMessages((state) =>
        areMessagesEqual(state, activeSession.messages) ? state : cloneMessages(activeSession.messages),
      );
      return;
    }

    if (activeSessionId === null && sessions.length === 0) {
      const introMessages = [createAssistantIntroMessage()];
      setMessages((state) => (areMessagesEqual(state, introMessages) ? state : introMessages));
    }
  }, [activeSession, activeSessionId, sessions.length]);

  useEffect(() => {
    if (diagnosticsLoaded.current || activeSessionId !== null || sessions.length > 0) {
      return;
    }
    diagnosticsLoaded.current = true;
    let cancelled = false;

    const loadDiagnostics = async () => {
      try {
        const diagnostics = await api.getDiagnostics();
        if (cancelled) {
          return;
        }
        const intro = buildDiagnosticsIntroMessage(diagnostics, createID);
        if (!intro) {
          return;
        }
        setMessages((state) => [...state, intro]);
        setDiagnosticPrompts(buildDiagnosticPrompts(diagnostics));
      } catch {
        // Ignore diagnostics preload failures.
      }
    };

    void loadDiagnostics();
    return () => {
      cancelled = true;
    };
  }, [activeSessionId, sessions.length]);

  const lastAssistant = useMemo(
    () => [...messages].reverse().find((message) => message.role === "assistant" && !message.isError),
    [messages],
  );

  const suggestionPool = useMemo(() => {
    const fromHints = lastAssistant?.hints ?? [];
    const fromResources = (lastAssistant?.resources ?? []).map((resource) => toDiagnosePrompt(resource));
    return dedupeStrings([...basePrompts, ...fromHints, ...fromResources]).slice(0, 10);
  }, [lastAssistant?.hints, lastAssistant?.resources]);

  const send = async (content: string, namespace?: string) => {
    const message = content.trim();
    if (message === "" || isLoading) {
      return;
    }

    const sessionID = activeSessionId ?? startNewSession();

    const userMessage: AssistantMessage = {
      id: createID(),
      role: "user",
      content: message,
      timestamp: new Date().toISOString(),
    };
    setMessages((state) => {
      const next = [...state, userMessage];
      saveSession(sessionID, next);
      return next;
    });
    setIsLoading(true);

    try {
      const response: AssistantResponse = await api.askAssistant(message, namespace);
      const assistantMessage: AssistantMessage = {
        id: createID(),
        role: "assistant",
        content: response.answer,
        timestamp: response.timestamp,
        query: message,
        hints: response.hints,
        resources: response.referencedResources,
        references: response.references,
      };
      setMessages((state) => {
        const next = [...state, assistantMessage];
        saveSession(sessionID, next);
        return next;
      });
    } catch (err) {
      setMessages((state) => {
        const errorMessage: AssistantMessage = {
          id: createID(),
          role: "assistant",
          isError: true,
          content: `Request failed: ${err instanceof Error ? err.message : "Unknown error"}`,
          timestamp: new Date().toISOString(),
          hints: ["Show cluster health", "What should I fix first?"],
        };
        const next = [...state, errorMessage];
        saveSession(sessionID, next);
        return next;
      });
    } finally {
      setIsLoading(false);
    }
  };

  const clear = () => {
    startNewSession();
    setMessages([]);
  };

  return {
    messages,
    isLoading,
    lastAssistant,
    suggestionPool,
    diagnosticPrompts,
    sessions,
    activeSessionId,
    selectSession,
    deleteSession,
    startNewSession,
    send,
    clear,
  };
}

function createAssistantIntroMessage(): AssistantMessage {
  return {
    id: createID(),
    timestamp: new Date().toISOString(),
    ...initialAssistantMessageTemplate,
  };
}



function dedupeStrings(values: readonly string[]): string[] {
  const out: string[] = [];
  for (const value of values) {
    const normalized = value.trim();
    if (normalized === "") {
      continue;
    }
    if (!out.includes(normalized)) {
      out.push(normalized);
    }
  }
  return out;
}

function toDiagnosePrompt(resource: string): string {
  const trimmed = resource.trim();
  if (trimmed === "") {
    return "Show cluster health";
  }
  const podName = trimmed.includes("/") ? (trimmed.split("/").pop() ?? trimmed) : trimmed;
  return `Diagnose ${podName}`;
}

function createID(): string {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function cloneMessages(messages: AssistantMessage[]): AssistantMessage[] {
  return messages.map((message) => ({
    ...message,
    hints: message.hints ? [...message.hints] : undefined,
    resources: message.resources ? [...message.resources] : undefined,
    references: message.references ? message.references.map((reference) => ({ ...reference })) : undefined,
  }));
}

function areMessagesEqual(left: AssistantMessage[], right: AssistantMessage[]): boolean {
  if (left.length !== right.length) {
    return false;
  }

  return left.every((message, index) => {
    const candidate = right[index];
    if (!candidate) {
      return false;
    }
    return JSON.stringify(message) === JSON.stringify(candidate);
  });
}
