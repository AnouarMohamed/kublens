import { useCallback, useMemo, useState } from "react";
import { useAsyncResource } from "../../app/hooks/useAsyncResource";
import { useAuthSession } from "../../context/AuthSessionContext";
import { KpiStrip } from "../../components/KpiStrip";
import { api } from "../../lib/api";
import type { Pod, ResourceRecord } from "../../types";

interface NamespaceRow {
  name: string;
  status: string;
  age: string;
  totalPods: number;
  runningPods: number;
  failedPods: number;
}

interface NamespacesPayload {
  namespaces: ResourceRecord[];
  pods: Pod[];
}

export default function Namespaces() {
  const { can, isLoading: authLoading } = useAuthSession();
  const [search, setSearch] = useState("");
  const canRead = can("read");

  const loadNamespaces = useCallback(async (signal: AbortSignal): Promise<NamespacesPayload> => {
    const [namespaceRows, podRows] = await Promise.all([api.getResources("namespaces", signal), api.getPods(signal)]);
    return { namespaces: namespaceRows.items, pods: podRows };
  }, []);

  const {
    data: { namespaces, pods },
    isLoading,
    error,
    load,
  } = useAsyncResource<NamespacesPayload>({
    loader: loadNamespaces,
    fallbackError: "Failed to load namespaces",
    initialData: { namespaces: [], pods: [] },
    autoLoad: !authLoading,
    enabled: canRead,
    disabledData: { namespaces: [], pods: [] },
    disabledError: "Authenticate to view namespaces.",
  });

  const rows = useMemo<NamespaceRow[]>(() => {
    return namespaces.map((namespace) => {
      const namespacePods = pods.filter((pod) => pod.namespace === namespace.name);
      return {
        name: namespace.name,
        status: namespace.status,
        age: namespace.age,
        totalPods: namespacePods.length,
        runningPods: namespacePods.filter((pod) => pod.status === "Running").length,
        failedPods: namespacePods.filter((pod) => pod.status === "Failed").length,
      };
    });
  }, [namespaces, pods]);

  const filtered = useMemo(() => {
    const query = search.trim().toLowerCase();
    if (query === "") {
      return rows;
    }
    return rows.filter((row) => `${row.name} ${row.status}`.toLowerCase().includes(query));
  }, [rows, search]);

  const totalPods = useMemo(() => rows.reduce((sum, row) => sum + row.totalPods, 0), [rows]);
  const totalFailedPods = useMemo(() => rows.reduce((sum, row) => sum + row.failedPods, 0), [rows]);

  return (
    <div className="space-y-5">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">Namespaces</h2>
          <p className="text-sm text-zinc-400 mt-1">Namespace boundaries with workload distribution context.</p>
        </div>
        <div className="flex gap-2">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search namespaces"
            className="field w-72"
          />
          <button onClick={() => void load()} disabled={isLoading || !canRead} className="btn">
            {isLoading ? "Loading" : "Refresh"}
          </button>
        </div>
      </header>

      <KpiStrip
        items={[
          { label: "Visible Namespaces", value: String(filtered.length), tone: "default" },
          { label: "Total Namespaces", value: String(rows.length), tone: "default" },
          { label: "Tracked Pods", value: String(totalPods), tone: "default" },
          { label: "Failed Pods", value: String(totalFailedPods), tone: totalFailedPods > 0 ? "critical" : "default" },
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
              <th className="px-4 py-3 font-semibold">Namespace</th>
              <th className="px-4 py-3 font-semibold">Status</th>
              <th className="px-4 py-3 font-semibold">Age</th>
              <th className="px-4 py-3 font-semibold">Pods</th>
              <th className="px-4 py-3 font-semibold">Running</th>
              <th className="px-4 py-3 font-semibold">Failed</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800 text-zinc-200">
            {filtered.map((row) => (
              <tr key={row.name} className="table-row">
                <td className="px-4 py-3 font-medium">{row.name}</td>
                <td className="px-4 py-3">{row.status || "-"}</td>
                <td className="px-4 py-3 text-zinc-400">{row.age || "-"}</td>
                <td className="px-4 py-3 text-zinc-400">{row.totalPods}</td>
                <td className="px-4 py-3 text-zinc-400">{row.runningPods}</td>
                <td className="px-4 py-3 text-zinc-400">{row.failedPods}</td>
              </tr>
            ))}
          </tbody>
        </table>

        {isLoading && <p className="px-4 py-8 text-center text-sm text-zinc-500">Loading namespaces...</p>}
        {!isLoading && filtered.length === 0 && (
          <p className="px-4 py-8 text-center text-sm text-zinc-500">No namespaces match the current filters.</p>
        )}
      </div>
    </div>
  );
}
