import { useCallback, useMemo, useState } from "react";
import { useAsyncResource } from "../../app/hooks/useAsyncResource";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import { useStreamRefresh } from "../../app/hooks/useStreamRefresh";
import { KpiStrip } from "../../components/KpiStrip";
import type { K8sEvent } from "../../types";

const EVENT_TYPES = ["All", "Warning", "Normal"] as const;

export default function Events() {
  const { can, isLoading: authLoading } = useAuthSession();
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<(typeof EVENT_TYPES)[number]>("All");
  const canRead = can("read");

  const loadEvents = useCallback((signal: AbortSignal) => api.getEvents(signal), []);

  const {
    data: events,
    isLoading,
    error,
    load,
  } = useAsyncResource<K8sEvent[]>({
    loader: loadEvents,
    fallbackError: "Failed to load events",
    initialData: [],
    autoLoad: !authLoading,
    enabled: canRead,
    disabledData: [],
    disabledError: "Authenticate to view cluster events.",
  });

  useStreamRefresh({
    enabled: canRead,
    eventTypes: ["k8s_event"],
    onEvent: () => {
      void load();
    },
  });

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    return events.filter((event) => {
      const matchesType = typeFilter === "All" || event.type === typeFilter;
      const matchesSearch =
        query === "" || `${event.type} ${event.reason} ${event.from} ${event.message}`.toLowerCase().includes(query);
      return matchesType && matchesSearch;
    });
  }, [events, search, typeFilter]);

  const warningsCount = useMemo(() => events.filter((event) => event.type === "Warning").length, [events]);
  const uniqueReasons = useMemo(() => new Set(events.map((event) => event.reason).filter(Boolean)).size, [events]);

  return (
    <div className="space-y-5">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">Events</h2>
          <p className="text-sm text-zinc-400 mt-1">Chronological cluster signals for troubleshooting and audits.</p>
        </div>
        <div className="flex gap-2">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search events"
            className="field w-72"
          />
          <select
            value={typeFilter}
            onChange={(event) => setTypeFilter(event.target.value as (typeof EVENT_TYPES)[number])}
            className="field"
          >
            {EVENT_TYPES.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
          <button onClick={() => void load()} disabled={isLoading || !canRead} className="btn">
            {isLoading ? "Loading" : "Refresh"}
          </button>
        </div>
      </header>

      <KpiStrip
        items={[
          { label: "Visible Events", value: String(filtered.length), tone: "default" },
          { label: "Total Events", value: String(events.length), tone: "default" },
          { label: "Warnings", value: String(warningsCount), tone: warningsCount > 0 ? "warning" : "default" },
          { label: "Unique Reasons", value: String(uniqueReasons), tone: "default" },
        ]}
        className="lg:grid-cols-4"
      />

      {error && (
        <div className="rounded-xl border border-zinc-700 bg-zinc-900/80 px-3 py-2 text-sm text-zinc-200">{error}</div>
      )}

      <div className="table-shell">
        <table className="min-w-full text-left text-sm">
          <thead className="table-head table-head-sticky">
            <tr>
              <th className="px-4 py-3 font-semibold">Type</th>
              <th className="px-4 py-3 font-semibold">Reason</th>
              <th className="px-4 py-3 font-semibold">Age</th>
              <th className="px-4 py-3 font-semibold">Source</th>
              <th className="px-4 py-3 font-semibold">Count</th>
              <th className="px-4 py-3 font-semibold">Message</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800 text-zinc-200">
            {filtered.map((event, index) => (
              <tr key={`${event.reason}-${event.age}-${index}`} className="table-row">
                <td className="px-4 py-3">
                  <span
                    className={`status-pill ${
                      event.type === "Warning"
                        ? "status-warning"
                        : event.type === "Normal"
                          ? "status-running"
                          : "status-unknown"
                    }`}
                  >
                    {event.type || "Unknown"}
                  </span>
                </td>
                <td className="px-4 py-3 font-medium">{event.reason || "-"}</td>
                <td className="px-4 py-3 text-zinc-400">{event.age || "-"}</td>
                <td className="px-4 py-3 text-zinc-400">{event.from || "-"}</td>
                <td className="px-4 py-3 text-zinc-400">{event.count ?? "-"}</td>
                <td className="px-4 py-3 text-zinc-300">{event.message || "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>

        {isLoading && <p className="px-4 py-8 text-center text-sm text-zinc-500">Loading events...</p>}
        {!isLoading && filtered.length === 0 && (
          <p className="px-4 py-8 text-center text-sm text-zinc-500">No events match the current filters.</p>
        )}
      </div>
    </div>
  );
}
