import { useCallback, useEffect, useState } from "react";
import { redactSensitiveText } from "../../../lib/security/redaction";
import type { AssistantMessage, ChatSession } from "../types";

export const CHAT_SESSIONS_STORAGE_KEY = "kubelens:chat-sessions";
const MAX_CHAT_SESSIONS = 10;

export function useChatSessions() {
  const [sessions, setSessions] = useState<ChatSession[]>(() => readStoredSessions());
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);

  useEffect(() => {
    if (sessions.length === 0) {
      if (activeSessionId !== null) {
        setActiveSessionId(null);
      }
      return;
    }

    if (!activeSessionId || !sessions.some((session) => session.id === activeSessionId)) {
      setActiveSessionId(sessions[0].id);
    }
  }, [activeSessionId, sessions]);

  useEffect(() => {
    writeStoredSessions(sessions);
  }, [sessions]);

  const startNewSession = useCallback(() => {
    const id = crypto.randomUUID();
    setActiveSessionId(id);
    return id;
  }, []);

  const selectSession = useCallback(
    (id: string) => {
      setActiveSessionId((current) => {
        if (sessions.some((session) => session.id === id)) {
          return id;
        }
        return current;
      });
    },
    [sessions],
  );

  const saveSession = useCallback((id: string, messages: AssistantMessage[]) => {
    const firstUserMessage = messages.find((message) => message.role === "user" && message.content.trim() !== "");
    if (!id || !firstUserMessage) {
      return;
    }

    setSessions((current) => {
      const existing = current.find((session) => session.id === id);
      const nextSession: ChatSession = {
        id,
        title: truncateTitle(firstUserMessage.content),
        startedAt: existing?.startedAt ?? normalizeTimestamp(firstUserMessage.timestamp),
        messages: cloneMessages(messages),
      };

      const next = pruneSessions([nextSession, ...current.filter((session) => session.id !== id)]);
      if (areSessionsEqual(current, next)) {
        return current;
      }
      return next;
    });
  }, []);

  const deleteSession = useCallback((id: string) => {
    setSessions((current) => current.filter((session) => session.id !== id));
  }, []);

  return {
    sessions,
    activeSessionId,
    startNewSession,
    selectSession,
    saveSession,
    deleteSession,
  };
}

function readStoredSessions(): ChatSession[] {
  try {
    const raw = window.localStorage.getItem(CHAT_SESSIONS_STORAGE_KEY);
    if (!raw) {
      return [];
    }

    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }

    return pruneSessions(parsed.map(normalizeSession).filter((session): session is ChatSession => session !== null));
  } catch {
    return [];
  }
}

function writeStoredSessions(sessions: ChatSession[]): void {
  window.localStorage.setItem(
    CHAT_SESSIONS_STORAGE_KEY,
    JSON.stringify(pruneSessions(sessions).map(sanitizeSessionForStorage)),
  );
}

function normalizeSession(value: unknown): ChatSession | null {
  if (!value || typeof value !== "object") {
    return null;
  }

  const candidate = value as Partial<ChatSession>;
  const id = typeof candidate.id === "string" ? candidate.id.trim() : "";
  const title = typeof candidate.title === "string" ? candidate.title.trim() : "";
  const startedAt = normalizeTimestamp(typeof candidate.startedAt === "string" ? candidate.startedAt : "");
  const messages = Array.isArray(candidate.messages)
    ? candidate.messages.filter(isAssistantMessage).map(cloneMessage)
    : [];

  if (id === "" || title === "" || messages.length === 0) {
    return null;
  }

  return {
    id,
    title: truncateTitle(title),
    startedAt,
    messages,
  };
}

function isAssistantMessage(value: unknown): value is AssistantMessage {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Partial<AssistantMessage>;
  return (
    typeof candidate.id === "string" &&
    (candidate.role === "user" || candidate.role === "assistant") &&
    typeof candidate.content === "string" &&
    typeof candidate.timestamp === "string"
  );
}

function pruneSessions(sessions: ChatSession[]): ChatSession[] {
  return [...sessions]
    .sort((left, right) => Date.parse(right.startedAt) - Date.parse(left.startedAt))
    .slice(0, MAX_CHAT_SESSIONS)
    .map(cloneSession);
}

function truncateTitle(value: string): string {
  const trimmed = value.trim();
  if (trimmed.length <= 60) {
    return trimmed;
  }
  return trimmed.slice(0, 60);
}

function normalizeTimestamp(value: string): string {
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return new Date().toISOString();
  }
  return new Date(parsed).toISOString();
}

function cloneMessages(messages: AssistantMessage[]): AssistantMessage[] {
  return messages.map(cloneMessage);
}

function cloneMessage(message: AssistantMessage): AssistantMessage {
  return {
    ...message,
    hints: message.hints ? [...message.hints] : undefined,
    resources: message.resources ? [...message.resources] : undefined,
    references: message.references ? message.references.map((reference) => ({ ...reference })) : undefined,
  };
}

function cloneSession(session: ChatSession): ChatSession {
  return {
    ...session,
    messages: cloneMessages(session.messages),
  };
}

function sanitizeSessionForStorage(session: ChatSession): ChatSession {
  return {
    ...session,
    title: redactSensitiveText(session.title),
    messages: session.messages.map(sanitizeMessageForStorage),
  };
}

function sanitizeMessageForStorage(message: AssistantMessage): AssistantMessage {
  return {
    ...message,
    content: redactSensitiveText(message.content),
    query: message.query ? redactSensitiveText(message.query) : undefined,
    hints: message.hints?.map(redactSensitiveText),
    resources: message.resources?.map(redactSensitiveText),
    references: message.references?.map((reference) => ({
      ...reference,
      title: redactSensitiveText(reference.title),
      snippet: reference.snippet ? redactSensitiveText(reference.snippet) : undefined,
    })),
  };
}

function areSessionsEqual(left: ChatSession[], right: ChatSession[]): boolean {
  if (left.length !== right.length) {
    return false;
  }

  return left.every((session, index) => {
    const candidate = right[index];
    if (!candidate) {
      return false;
    }
    return (
      session.id === candidate.id &&
      session.title === candidate.title &&
      session.startedAt === candidate.startedAt &&
      areMessagesEqual(session.messages, candidate.messages)
    );
  });
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
