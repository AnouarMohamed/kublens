import StatusText from "../../../components/pods/PodStatusBadge";
import type { Pod } from "../../../types";

/**
 * Pod inventory table with operational actions.
 */
interface PodsTableProps {
  pods: Pod[];
  canRead: boolean;
  canWrite: boolean;
  isLoading: boolean;
  confirmingDeleteId: string | null;
  onOpenDetail: (namespace: string, podName: string) => Promise<void>;
  onOpenLogs: (namespace: string, podName: string) => void;
  onOpenSnapshot: (namespace: string, podName: string) => Promise<void>;
  onRestartPod: (namespace: string, podName: string) => Promise<void>;
  onRequestDelete: (pod: Pod) => Promise<void>;
}

export function PodsTable({
  pods,
  canRead,
  canWrite,
  isLoading,
  confirmingDeleteId,
  onOpenDetail,
  onOpenLogs,
  onOpenSnapshot,
  onRestartPod,
  onRequestDelete,
}: PodsTableProps) {
  return (
    <div className="table-shell">
      <table className="min-w-full text-left text-sm">
        <thead className="table-head table-head-sticky">
          <tr>
            <th className="px-4 py-3 font-semibold">Pod</th>
            <th className="px-4 py-3 font-semibold">Namespace</th>
            <th className="px-4 py-3 font-semibold">Status</th>
            <th className="px-4 py-3 font-semibold">CPU</th>
            <th className="px-4 py-3 font-semibold">Memory</th>
            <th className="px-4 py-3 font-semibold">Age</th>
            <th className="px-4 py-3 font-semibold">Restarts</th>
            <th className="px-4 py-3 font-semibold">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-800 text-zinc-200">
          {pods.map((pod) => (
            <tr key={pod.id} className="table-row">
              <td className="px-4 py-3">
                <button
                  onClick={() => void onOpenDetail(pod.namespace, pod.name)}
                  className="text-left hover:underline"
                >
                  <p className="font-medium">{pod.name}</p>
                </button>
              </td>
              <td className="px-4 py-3 text-zinc-400">{pod.namespace}</td>
              <td className="px-4 py-3">
                <StatusText status={pod.status} />
              </td>
              <td className="px-4 py-3 text-zinc-400">{pod.cpu}</td>
              <td className="px-4 py-3 text-zinc-400">{pod.memory}</td>
              <td className="px-4 py-3 text-zinc-400">{pod.age}</td>
              <td className="px-4 py-3 text-zinc-400">{pod.restarts}</td>
              <td className="px-4 py-3">
                <div className="flex gap-2">
                  <button
                    onClick={() => void onOpenLogs(pod.namespace, pod.name)}
                    className="text-xs font-mono text-[#444444] hover:text-[#e8e8e8] transition-colors"
                    disabled={!canRead}
                  >
                    Logs
                  </button>
                  <button
                    onClick={() => void onOpenSnapshot(pod.namespace, pod.name)}
                    className="text-xs font-mono text-[#444444] hover:text-[#e8e8e8] transition-colors"
                    disabled={!canRead}
                  >
                    Snapshot
                  </button>
                  <button
                    onClick={() => void onRestartPod(pod.namespace, pod.name)}
                    className="text-xs font-mono text-[#444444] hover:text-[#e8e8e8] transition-colors"
                    disabled={!canWrite}
                  >
                    Restart
                  </button>
                  <button
                    onClick={() => void onRequestDelete(pod)}
                    disabled={!canWrite}
                    className={`text-xs font-mono transition-colors ${
                      confirmingDeleteId === pod.id ? "text-[#ff4444]" : "text-[#ff4444]/50 hover:text-[#ff4444]"
                    }`}
                  >
                    {confirmingDeleteId === pod.id ? "Confirm?" : "Delete"}
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {isLoading && <p className="px-4 py-8 text-center text-sm text-zinc-500">Loading pods...</p>}
      {!isLoading && pods.length === 0 && (
        <p className="px-4 py-8 text-center text-sm text-zinc-500">No pods match the current filters.</p>
      )}
    </div>
  );
}
