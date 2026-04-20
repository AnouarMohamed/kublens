import { Trash2 } from "lucide-react";
import type { ChatSession } from "../types";
import { toDiagnosePrompt } from "../utils";
import { InfoRow } from "./OpsAssistantPrimitives";

interface OpsAssistantSidebarProps {
  sessions: ChatSession[];
  activeSessionId: string | null;
  quickActions: Array<{ label: string; prompt: string }>;
  decisionPrompts: string[];
  assistantReplies: number;
  isLoading: boolean;
  referencesCount: number;
  selectedNamespace: string;
  latestResources: string[];
  onStartNewSession: () => void;
  onSelectSession: (id: string) => void;
  onDeleteSession: (id: string) => void;
  onRunPrompt: (prompt: string) => void;
}

export function OpsAssistantSidebar({
  sessions,
  activeSessionId,
  quickActions,
  decisionPrompts,
  assistantReplies,
  isLoading,
  referencesCount,
  selectedNamespace,
  latestResources,
  onStartNewSession,
  onSelectSession,
  onDeleteSession,
  onRunPrompt,
}: OpsAssistantSidebarProps) {
  return (
    <aside className="hidden xl:flex xl:flex-col bg-zinc-900/80">
      <div className="border-b border-zinc-700 px-4 py-4">
        <button onClick={onStartNewSession} className="btn-sm w-full justify-center">
          New Chat
        </button>

        <div className="mt-4">
          <p className="text-xs uppercase tracking-wide text-zinc-500 font-semibold">Recent sessions</p>
          <div className="mt-3 space-y-1">
            {sessions.map((session) => {
              const isActive = session.id === activeSessionId;
              return (
                <div
                  key={session.id}
                  className={`group flex items-start gap-2 border-l-2 px-2 py-2 ${
                    isActive ? "border-l-[var(--accent)] bg-zinc-800/70" : "border-l-transparent hover:bg-zinc-800/50"
                  }`}
                >
                  <button onClick={() => onSelectSession(session.id)} className="min-w-0 flex-1 text-left">
                    <p className="truncate text-sm text-zinc-200">{session.title}</p>
                    <p className="mt-0.5 text-[11px] text-zinc-500">{formatRelativeTimestamp(session.startedAt)}</p>
                  </button>
                  <button
                    onClick={() => onDeleteSession(session.id)}
                    className="rounded-md p-1 text-zinc-500 opacity-0 transition group-hover:opacity-100 hover:bg-zinc-800 hover:text-zinc-300"
                    aria-label={`Delete session ${session.title}`}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              );
            })}
            {sessions.length === 0 && <p className="text-xs text-zinc-500">No saved conversations yet.</p>}
          </div>
        </div>
      </div>

      <div className="border-b border-zinc-700 px-4 py-4">
        <p className="text-xs uppercase tracking-wide text-zinc-500 font-semibold">Interactive follow-ups</p>
        <div className="mt-3 space-y-2">
          {quickActions.map((action) => (
            <button
              key={action.label}
              onClick={() => onRunPrompt(action.prompt)}
              disabled={isLoading || action.prompt.trim() === ""}
              className="w-full rounded-md border border-zinc-700 bg-zinc-800/60 px-2 py-1.5 text-left text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50"
            >
              {action.label}
            </button>
          ))}
        </div>
      </div>

      <div className="px-4 py-4 border-b border-zinc-700">
        <p className="text-xs uppercase tracking-wide text-zinc-500 font-semibold">Decision support</p>
        <div className="mt-3 space-y-2">
          {decisionPrompts.map((prompt) => (
            <button
              key={prompt}
              onClick={() => onRunPrompt(prompt)}
              disabled={isLoading}
              className="w-full rounded-md border border-zinc-700 bg-zinc-800/60 px-2 py-1.5 text-left text-xs text-zinc-300 hover:bg-zinc-800 disabled:opacity-50"
            >
              {prompt}
            </button>
          ))}
        </div>
      </div>

      <div className="px-4 py-4 border-b border-zinc-700">
        <p className="text-xs uppercase tracking-wide text-zinc-500 font-semibold">Session</p>
        <div className="mt-3 space-y-2 text-xs text-zinc-300">
          <InfoRow label="Assistant replies" value={String(assistantReplies)} />
          <InfoRow label="Pending" value={isLoading ? "Yes" : "No"} />
          <InfoRow label="References" value={String(referencesCount)} />
          <InfoRow label="Namespace" value={selectedNamespace} />
        </div>
      </div>

      <div className="px-4 py-4 overflow-auto">
        <p className="text-xs uppercase tracking-wide text-zinc-500 font-semibold">Latest referenced resources</p>
        <div className="mt-3 flex flex-wrap gap-2">
          {latestResources.map((resource) => (
            <button
              key={`ctx-${resource}`}
              onClick={() => onRunPrompt(toDiagnosePrompt(resource))}
              className="rounded-md border border-zinc-700 bg-zinc-800/60 px-2 py-1 text-[11px] text-zinc-300 hover:bg-zinc-800"
            >
              {resource}
            </button>
          ))}
          {latestResources.length === 0 && <p className="text-xs text-zinc-500">No referenced resources yet.</p>}
        </div>
      </div>
    </aside>
  );
}

function formatRelativeTimestamp(value: string): string {
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return value;
  }

  const diffMs = parsed - Date.now();
  const diffSeconds = Math.round(diffMs / 1000);
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

  if (Math.abs(diffSeconds) < 60) {
    return formatter.format(diffSeconds, "second");
  }
  const diffMinutes = Math.round(diffSeconds / 60);
  if (Math.abs(diffMinutes) < 60) {
    return formatter.format(diffMinutes, "minute");
  }
  const diffHours = Math.round(diffMinutes / 60);
  if (Math.abs(diffHours) < 24) {
    return formatter.format(diffHours, "hour");
  }
  const diffDays = Math.round(diffHours / 24);
  return formatter.format(diffDays, "day");
}
