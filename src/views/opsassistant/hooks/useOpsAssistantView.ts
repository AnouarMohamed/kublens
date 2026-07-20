import { useCallback, useEffect, useMemo, useRef, useState, type MutableRefObject } from "react";
import { api } from "../../../lib/api";
import { redactSensitiveText } from "../../../lib/security/redaction";
import { useAssistantChat } from "./useAssistantChat";
import type { AssistantMessage as Message, ChatSession } from "../types";
import { applyIntentToPrompt, buildFollowUpPrompt } from "../utils";
import { ASSISTANT_DRAFT_KEY, type AssistantIntent } from "../constants";

type ReferenceFeedbackValue = "helpful" | "not_helpful" | "pending";

/**
 * State and interactions for the Ops Assistant view shell.
 */
interface UseOpsAssistantViewResult {
  messages: Message[];
  isLoading: boolean;
  lastAssistant: Message | undefined;
  suggestionPool: string[];
  diagnosticPrompts: string[];
  sessions: ChatSession[];
  activeSessionId: string | null;
  input: string;
  intentMode: AssistantIntent;
  copiedMessageID: string | null;
  referenceFeedback: Record<string, ReferenceFeedbackValue>;
  namespaces: string[];
  selectedNamespace: string;
  scrollRef: MutableRefObject<HTMLDivElement | null>;
  quickActions: Array<{ label: string; prompt: string }>;
  decisionPrompts: string[];
  assistantReplies: number;
  setInput: (value: string) => void;
  setIntentMode: (intent: AssistantIntent) => void;
  setSelectedNamespace: (namespace: string) => void;
  startNewSession: () => void;
  selectSession: (id: string) => void;
  deleteSession: (id: string) => void;
  submit: (promptOverride?: string) => Promise<void>;
  copyMessage: (message: Message) => Promise<void>;
  submitReferenceFeedback: (message: Message, url: string, helpful: boolean) => Promise<void>;
  resetSession: () => void;
}

/**
 * Combines assistant chat data with view-only state and actions.
 *
 * @returns View-model and handlers for Ops Assistant rendering.
 */
export function useOpsAssistantView(): UseOpsAssistantViewResult {
  const {
    messages,
    isLoading,
    lastAssistant,
    suggestionPool,
    diagnosticPrompts,
    sessions,
    activeSessionId,
    selectSession: selectStoredSession,
    deleteSession: deleteStoredSession,
    send,
    clear,
  } = useAssistantChat();
  const [input, setInputState] = useState("");
  const [intentMode, setIntentModeState] = useState<AssistantIntent>("triage");
  const [copiedMessageID, setCopiedMessageID] = useState<string | null>(null);
  const [referenceFeedback, setReferenceFeedback] = useState<Record<string, ReferenceFeedbackValue>>({});
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedNamespace, setSelectedNamespaceState] = useState("All");
  const scrollRef = useRef<HTMLDivElement>(null);

  const setInput = useCallback((value: string) => {
    setInputState(value);
  }, []);

  const setIntentMode = useCallback((intent: AssistantIntent) => {
    setIntentModeState(intent);
  }, []);

  const setSelectedNamespace = useCallback((namespace: string) => {
    setSelectedNamespaceState(namespace);
  }, []);

  const startNewSession = useCallback(() => {
    clear();
    setInputState("");
    setReferenceFeedback({});
    setCopiedMessageID(null);
  }, [clear]);

  const selectSession = useCallback(
    (id: string) => {
      selectStoredSession(id);
      setReferenceFeedback({});
      setCopiedMessageID(null);
    },
    [selectStoredSession],
  );

  const deleteSession = useCallback(
    (id: string) => {
      deleteStoredSession(id);
      setReferenceFeedback({});
      setCopiedMessageID(null);
    },
    [deleteStoredSession],
  );

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, isLoading]);

  useEffect(() => {
    const saved = window.localStorage.getItem(ASSISTANT_DRAFT_KEY);
    if (!saved) {
      return;
    }
    setInputState(saved);
  }, []);

  useEffect(() => {
    window.localStorage.setItem(ASSISTANT_DRAFT_KEY, redactSensitiveText(input));
  }, [input]);

  useEffect(() => {
    let cancelled = false;
    const loadNamespaces = async () => {
      try {
        const rows = await api.getNamespaces();
        if (cancelled) {
          return;
        }
        setNamespaces(rows);
      } catch {
        // Namespace context is optional for assistant prompts.
      }
    };
    void loadNamespaces();
    return () => {
      cancelled = true;
    };
  }, []);

  const quickActions = useMemo(
    () => [
      {
        label: "Explain Simpler",
        prompt: buildFollowUpPrompt("Explain this in simpler terms", lastAssistant?.content),
      },
      {
        label: "Step-by-step runbook",
        prompt: buildFollowUpPrompt("Give me a step-by-step runbook", lastAssistant?.content),
      },
      {
        label: "kubectl only",
        prompt: buildFollowUpPrompt("Give only kubectl commands to verify and fix", lastAssistant?.content),
      },
      {
        label: "Rollback and blast radius",
        prompt: buildFollowUpPrompt("Include rollback steps and blast radius assessment", lastAssistant?.content),
      },
    ],
    [lastAssistant?.content],
  );

  const decisionPrompts = useMemo(() => {
    const prompts: string[] = [];
    if ((lastAssistant?.resources?.length ?? 0) > 0) {
      prompts.push(`Create an operator checklist for ${lastAssistant?.resources?.[0]}`);
    }
    if ((lastAssistant?.references?.length ?? 0) > 0) {
      prompts.push("Summarize evidence quality from referenced docs before acting");
    }
    prompts.push("Which action gives the highest risk reduction in the next 10 minutes?");
    prompts.push("What should I monitor right after applying the fix?");
    return prompts.slice(0, 4);
  }, [lastAssistant?.references?.length, lastAssistant?.resources]);

  const submit = useCallback(
    async (promptOverride?: string) => {
      const content = (promptOverride ?? input).trim();
      if (content === "" || isLoading) {
        return;
      }
      setInputState("");
      const preparedPrompt = applyIntentToPrompt(content, intentMode);
      await send(preparedPrompt, selectedNamespace === "All" ? undefined : selectedNamespace);
    },
    [input, intentMode, isLoading, selectedNamespace, send],
  );

  const copyMessage = useCallback(async (message: Message) => {
    try {
      await navigator.clipboard.writeText(message.content);
      setCopiedMessageID(message.id);
      window.setTimeout(() => setCopiedMessageID(null), 1200);
    } catch {
      // no-op
    }
  }, []);

  const submitReferenceFeedback = useCallback(async (message: Message, url: string, helpful: boolean) => {
    if (message.role !== "assistant") {
      return;
    }
    const key = `${message.id}::${url}`;
    setReferenceFeedback((current) => ({ ...current, [key]: "pending" }));
    try {
      await api.submitAssistantReferenceFeedback({
        query: message.query ?? "",
        url,
        helpful,
      });
      setReferenceFeedback((current) => ({
        ...current,
        [key]: helpful ? "helpful" : "not_helpful",
      }));
    } catch {
      setReferenceFeedback((current) => {
        const next = { ...current };
        delete next[key];
        return next;
      });
    }
  }, []);

  const resetSession = useCallback(() => {
    clear();
    setInputState("");
    setReferenceFeedback({});
  }, [clear]);

  const assistantReplies = useMemo(() => messages.filter((m) => m.role === "assistant").length, [messages]);

  return {
    messages,
    isLoading,
    lastAssistant,
    suggestionPool,
    diagnosticPrompts,
    sessions,
    activeSessionId,
    input,
    intentMode,
    copiedMessageID,
    referenceFeedback,
    namespaces,
    selectedNamespace,
    scrollRef,
    quickActions,
    decisionPrompts,
    assistantReplies,
    setInput,
    setIntentMode,
    setSelectedNamespace,
    startNewSession,
    selectSession,
    deleteSession,
    submit,
    copyMessage,
    submitReferenceFeedback,
    resetSession,
  };
}
