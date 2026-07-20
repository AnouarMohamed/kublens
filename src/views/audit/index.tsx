import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ShieldCheck } from "lucide-react";
import { api } from "../../lib/api";
import { parseStreamEvent } from "../../lib/api/stream";
import type { AuditEntry, AuditVerification } from "../../types";

const MAX_ROWS = 3000;
const ROW_HEIGHT = 54;
const ROW_OVERSCAN = 8;
const DEFAULT_VIEWPORT_HEIGHT = 520;
const BASE_RECONNECT_MS = 1000;
const MAX_RECONNECT_MS = 30000;

interface VirtualRows {
  visibleRows: AuditEntry[];
  topPadding: number;
  bottomPadding: number;
}

export default function AuditView() {
  const [rows, setRows] = useState<AuditEntry[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [verificationError, setVerificationError] = useState<string | null>(null);
  const [verifyingID, setVerifyingID] = useState<string | null>(null);
  const [verificationByID, setVerificationByID] = useState<Record<string, AuditVerification>>({});
  const [filter, setFilter] = useState("");
  const [livePaused, setLivePaused] = useState(false);
  const [bufferedCount, setBufferedCount] = useState(0);
  const [reconnectDelayMs, setReconnectDelayMs] = useState<number | null>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(DEFAULT_VIEWPORT_HEIGHT);
  const tableScrollRef = useRef<HTMLDivElement | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const reconnectAttemptRef = useRef(0);
  const pausedBufferRef = useRef<AuditEntry[]>([]);
  const livePausedRef = useRef(livePaused);

  useEffect(() => {
    livePausedRef.current = livePaused;
  }, [livePaused]);

  const filteredRows = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (needle === "") {
      return rows;
    }
    return rows.filter((row) => {
      const text = `${row.method} ${row.path} ${row.user ?? ""} ${row.role ?? ""} ${row.action ?? ""}`.toLowerCase();
      return text.includes(needle);
    });
  }, [rows, filter]);

  const virtualRows = useMemo<VirtualRows>(() => {
    if (filteredRows.length === 0) {
      return { visibleRows: [], topPadding: 0, bottomPadding: 0 };
    }

    const visibleCount = Math.max(1, Math.ceil(viewportHeight / ROW_HEIGHT) + ROW_OVERSCAN * 2);
    const startIndex = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - ROW_OVERSCAN);
    const endIndex = Math.min(filteredRows.length, startIndex + visibleCount);
    const topPadding = startIndex * ROW_HEIGHT;
    const bottomPadding = Math.max(0, (filteredRows.length - endIndex) * ROW_HEIGHT);

    return {
      visibleRows: filteredRows.slice(startIndex, endIndex),
      topPadding,
      bottomPadding,
    };
  }, [filteredRows, scrollTop, viewportHeight]);

  const flushBufferedRows = useCallback(() => {
    if (pausedBufferRef.current.length === 0) {
      return;
    }
    const bufferedRows = [...pausedBufferRef.current].reverse();
    pausedBufferRef.current = [];
    setBufferedCount(0);
    setRows((current) => [...bufferedRows, ...current].slice(0, MAX_ROWS));
  }, []);

  const loadAudit = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getAuditLog(300);
      setRows(data.items);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load audit log");
    } finally {
      setLoading(false);
    }
  }, []);

  const verifyEntry = useCallback(async (entry: AuditEntry) => {
    setVerifyingID(entry.id);
    setVerificationError(null);
    try {
      const verification = await api.verifyAuditEntry(entry.id);
      setVerificationByID((current) => ({
        ...current,
        [entry.id]: verification,
      }));
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : "Failed to verify audit entry");
    } finally {
      setVerifyingID(null);
    }
  }, []);

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    void loadAudit();
  }, [loadAudit]);

  useEffect(() => {
    if (livePaused) {
      return;
    }
    flushBufferedRows();
  }, [flushBufferedRows, livePaused]);

  useEffect(() => {
    const updateViewportHeight = () => {
      if (!tableScrollRef.current) {
        return;
      }
      setViewportHeight(tableScrollRef.current.clientHeight || DEFAULT_VIEWPORT_HEIGHT);
    };
    updateViewportHeight();
    window.addEventListener("resize", updateViewportHeight);
    return () => window.removeEventListener("resize", updateViewportHeight);
  }, []);

  useEffect(() => {
    let cancelled = false;

    const scheduleReconnect = () => {
      if (cancelled) {
        return;
      }
      const attempt = reconnectAttemptRef.current;
      const exponential = Math.min(MAX_RECONNECT_MS, BASE_RECONNECT_MS * 2 ** attempt);
      const jitter = Math.floor(Math.random() * 250);
      const delay = Math.min(MAX_RECONNECT_MS, exponential + jitter);
      reconnectAttemptRef.current = Math.min(attempt + 1, 10);
      setReconnectDelayMs(delay);
      clearReconnectTimer();
      reconnectTimerRef.current = window.setTimeout(() => {
        setReconnectDelayMs(null);
        void connectStream();
      }, delay);
    };

    const connectStream = async () => {
      clearReconnectTimer();
      if (cancelled) {
        return;
      }

      try {
        const session = await api.getAuthSession();
        if (cancelled) {
          return;
        }
        if (session.enabled && !session.authenticated) {
          setConnected(false);
          setError("Authenticate from Profile to enable live stream.");
          scheduleReconnect();
          return;
        }
      } catch {
        // Keep trying to open stream in local mode.
      }

      const socket = new WebSocket(api.getStreamWSURL());
      socketRef.current = socket;

      socket.onopen = () => {
        if (cancelled) {
          return;
        }
        reconnectAttemptRef.current = 0;
        setReconnectDelayMs(null);
        setConnected(true);
        setError(null);
      };

      socket.onmessage = (event) => {
        const payload = parseStreamEvent<AuditEntry>(event.data);
        if (!payload || payload.type !== "audit") {
          return;
        }
        if (livePausedRef.current) {
          pausedBufferRef.current.push(payload.payload);
          if (pausedBufferRef.current.length > MAX_ROWS) {
            pausedBufferRef.current = pausedBufferRef.current.slice(-MAX_ROWS);
          }
          setBufferedCount(pausedBufferRef.current.length);
          return;
        }
        setRows((current) => [payload.payload, ...current].slice(0, MAX_ROWS));
      };

      socket.onerror = () => {
        if (cancelled) {
          return;
        }
        setConnected(false);
      };

      socket.onclose = () => {
        if (cancelled) {
          return;
        }
        setConnected(false);
        scheduleReconnect();
      };
    };

    void connectStream();
    return () => {
      cancelled = true;
      clearReconnectTimer();
      socketRef.current?.close();
      socketRef.current = null;
      setConnected(false);
      setReconnectDelayMs(null);
    };
  }, [clearReconnectTimer]);

  return (
    <div className="space-y-4">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">Audit Trail</h2>
          <p className="text-sm text-zinc-500 mt-1">Live operator activity stream with request-level attribution.</p>
        </div>
        <div className="flex items-center gap-2">
          <span
            className={`rounded-full border px-3 py-1 text-xs font-medium ${connected ? "border-emerald-500/40 text-emerald-300" : "border-zinc-600 text-zinc-400"}`}
          >
            {connected ? "Stream connected" : "Stream disconnected"}
          </span>
          <button onClick={() => setLivePaused((value) => !value)} className="btn">
            {livePaused ? "Resume Live" : "Pause Live"}
          </button>
          <button onClick={() => void loadAudit()} className="btn">
            Refresh
          </button>
        </div>
      </header>

      <section className="surface p-4 space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto_auto_auto] gap-2">
          <input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="Filter by user, action, path, method"
            className="field"
          />
          <div className="rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2 text-xs text-zinc-400">
            Showing <span className="text-zinc-100">{filteredRows.length}</span> rows
          </div>
          <div className="rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2 text-xs text-zinc-400">
            Cached <span className="text-zinc-100">{rows.length}</span>
          </div>
          <div className="rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2 text-xs text-zinc-400">
            Buffered <span className="text-zinc-100">{bufferedCount}</span>
          </div>
        </div>

        {livePaused && (
          <p className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-200">
            Live feed paused. Incoming entries are buffered and will be merged when resumed.
          </p>
        )}

        {!connected && reconnectDelayMs !== null && (
          <p className="rounded-lg border border-zinc-700 bg-zinc-800/60 px-3 py-2 text-sm text-zinc-300">
            Stream reconnect scheduled in ~{Math.ceil(reconnectDelayMs / 1000)}s.
          </p>
        )}

        {error && (
          <p className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">{error}</p>
        )}
        {verificationError && (
          <p className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            {verificationError}
          </p>
        )}

        <div
          ref={tableScrollRef}
          onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
          className="overflow-auto rounded-xl border border-zinc-800 max-h-[65vh]"
        >
          <table className="min-w-full text-sm">
            <thead className="bg-zinc-900 sticky top-0 z-10">
              <tr className="text-left text-xs uppercase tracking-wide text-zinc-500">
                <th className="px-3 py-2">Time</th>
                <th className="px-3 py-2">User</th>
                <th className="px-3 py-2">Method</th>
                <th className="px-3 py-2">Action</th>
                <th className="px-3 py-2">Path</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Integrity</th>
                <th className="px-3 py-2">Duration</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr>
                  <td colSpan={8} className="px-3 py-6 text-center text-zinc-500">
                    Loading audit entries...
                  </td>
                </tr>
              )}
              {!loading && virtualRows.topPadding > 0 && (
                <tr aria-hidden>
                  <td colSpan={8} style={{ height: virtualRows.topPadding, padding: 0 }} />
                </tr>
              )}
              {!loading &&
                virtualRows.visibleRows.map((row) => (
                  <tr key={row.id} className="h-11 border-t border-zinc-800 text-zinc-300">
                    <td className="px-3 py-2 text-xs text-zinc-400">{formatTime(row.timestamp)}</td>
                    <td className="px-3 py-2">
                      <span className="text-zinc-100">{row.user ?? "unknown"}</span>
                      {row.role && <span className="ml-2 text-xs text-zinc-500">({row.role})</span>}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs">{row.method}</td>
                    <td className="px-3 py-2">{row.action ?? "-"}</td>
                    <td className="px-3 py-2 font-mono text-xs text-zinc-400">{row.path}</td>
                    <td className="px-3 py-2">
                      <span
                        className={`rounded-md px-2 py-0.5 text-xs ${row.success ? "bg-emerald-500/15 text-emerald-300" : "bg-red-500/15 text-red-300"}`}
                      >
                        {row.status}
                      </span>
                    </td>
                    <td className="px-3 py-2">
                      <AuditIntegrityCell
                        row={row}
                        verification={verificationByID[row.id]}
                        verifying={verifyingID === row.id}
                        onVerify={() => void verifyEntry(row)}
                      />
                    </td>
                    <td className="px-3 py-2 text-xs text-zinc-400">{row.durationMs}ms</td>
                  </tr>
                ))}
              {!loading && virtualRows.bottomPadding > 0 && (
                <tr aria-hidden>
                  <td colSpan={8} style={{ height: virtualRows.bottomPadding, padding: 0 }} />
                </tr>
              )}
              {!loading && filteredRows.length === 0 && (
                <tr>
                  <td colSpan={8} className="px-3 py-8 text-center text-zinc-500">
                    No matching audit entries.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function AuditIntegrityCell({
  row,
  verification,
  verifying,
  onVerify,
}: {
  row: AuditEntry;
  verification?: AuditVerification;
  verifying: boolean;
  onVerify: () => void;
}) {
  if (!row.hash) {
    return <span className="text-xs text-zinc-500">unhashed</span>;
  }

  return (
    <div className="flex min-w-[130px] items-center gap-2">
      <button
        type="button"
        onClick={onVerify}
        disabled={verifying}
        className="btn-sm inline-flex items-center gap-1.5 px-2 py-0.5"
      >
        <ShieldCheck size={12} />
        {verifying ? "Checking" : "Verify"}
      </button>
      <div className="min-w-0">
        <p
          className={`text-[11px] ${verification ? (verification.ok ? "text-emerald-300" : "text-red-300") : "text-zinc-500"}`}
        >
          {verification ? (verification.ok ? "verified" : verification.message) : shortHash(row.hash)}
        </p>
      </div>
    </div>
  );
}

function formatTime(value: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function shortHash(value: string): string {
  const trimmed = value.trim();
  return trimmed.length > 12 ? trimmed.slice(0, 12) : trimmed;
}
