import { useEffect, useRef } from "react";

/**
 * Modal for snapshot and streaming pod logs.
 */
interface PodLogsModalProps {
  logLines: string[];
  logPodName: string;
  logStreaming: boolean;
  logError: string | null;
  onStop: () => void;
  onClose: () => void;
}

export function PodLogsModal({ logLines, logPodName, logStreaming, logError, onStop, onClose }: PodLogsModalProps) {
  const preRef = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    if (!preRef.current) {
      return;
    }
    preRef.current.scrollTop = preRef.current.scrollHeight;
  }, [logLines]);

  if (logPodName.trim() === "") {
    return null;
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-4xl app-shell">
        <header className="border-b border-zinc-800 px-4 py-3 flex items-center justify-between">
          <div>
            <p className="text-sm font-semibold text-zinc-100">Pod Logs</p>
            <p className="text-xs text-zinc-500">{logPodName}</p>
            <p className="text-[11px] text-zinc-500">{logStreaming ? "Streaming live output" : "Snapshot"}</p>
          </div>
          <div className="flex gap-2">
            {logStreaming && (
              <button onClick={onStop} className="btn-sm border-zinc-600">
                Stop
              </button>
            )}
            <button onClick={onClose} className="btn-sm">
              Close
            </button>
          </div>
        </header>
        {logError && (
          <div className="border-b border-zinc-800 px-4 py-2 text-xs text-rose-300 bg-rose-500/10">{logError}</div>
        )}
        <pre
          ref={preRef}
          className="max-h-[60vh] overflow-auto p-4 text-xs leading-relaxed text-zinc-200 bg-zinc-900/60"
        >
          {logLines.join("\n")}
        </pre>
      </div>
    </div>
  );
}
