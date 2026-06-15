import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, Play, RefreshCw } from "lucide-react";
import * as THREE from "three";
import { api } from "../../lib/api";
import type { GhostFramePod, GhostSimulationResult, GhostTimelineFrame, GhostTopology } from "../../types";

const DEFAULT_HORIZON_SECONDS = 900;

interface VectorLike {
  x: number;
  y: number;
  z: number;
}

interface SceneLike {
  add: (item: unknown) => void;
}

interface DisposableMeshLike {
  geometry: { dispose: () => void };
  material: { dispose: () => void } | Array<{ dispose: () => void }>;
}

export default function GhostMode() {
  const [topology, setTopology] = useState<GhostTopology | null>(null);
  const [simulation, setSimulation] = useState<GhostSimulationResult | null>(null);
  const [selectedNode, setSelectedNode] = useState("");
  const [horizonSeconds, setHorizonSeconds] = useState(DEFAULT_HORIZON_SECONDS);
  const [frameIndex, setFrameIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [isRunning, setIsRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadTopology = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const next = await api.getGhostTopology();
      setTopology(next);
      setSelectedNode((current) => {
        if (current && next.nodes.some((node) => node.name === current)) {
          return current;
        }
        return next.nodes.find((node) => !node.unschedulable)?.name || next.nodes[0]?.name || "";
      });
      setSimulation(null);
      setFrameIndex(0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load ghost topology");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadTopology();
  }, [loadTopology]);

  const runSimulation = useCallback(async () => {
    if (!selectedNode) {
      setError("Select a node before running a simulation.");
      return;
    }
    setIsRunning(true);
    setError(null);
    try {
      const result = await api.simulateGhostScenario({
        action: "node_drain",
        nodeName: selectedNode,
        horizonSeconds,
      });
      setSimulation(result);
      setFrameIndex(Math.max(0, result.frames.length - 1));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to run ghost simulation");
    } finally {
      setIsRunning(false);
    }
  }, [horizonSeconds, selectedNode]);

  const activeFrame = simulation?.frames[frameIndex] ?? null;
  const visiblePods = useMemo(() => activeFrame?.pods ?? topology?.pods ?? [], [activeFrame?.pods, topology?.pods]);
  const evictedPods = useMemo(() => visiblePods.filter((pod) => pod.status === "Pending").length, [visiblePods]);
  const movedPods = useMemo(() => countMovedPods(topology, activeFrame), [activeFrame, topology]);

  return (
    <div className="space-y-5 text-zinc-100">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">Simulation</p>
          <h1 className="mt-1 text-xl font-semibold">Ghost Mode</h1>
        </div>
        <button
          type="button"
          onClick={() => void loadTopology()}
          disabled={isLoading}
          className="btn-sm inline-flex gap-2"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </header>

      {error && <div className="border border-red-500/35 bg-red-500/10 px-3 py-2 text-sm text-red-100">{error}</div>}

      <section className="grid gap-5 xl:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <section className="surface p-4 space-y-3">
            <div>
              <label className="text-[11px] uppercase tracking-[0.16em] text-zinc-500" htmlFor="ghost-node">
                Node
              </label>
              <select
                id="ghost-node"
                className="field mt-2 w-full"
                value={selectedNode}
                onChange={(event) => setSelectedNode(event.target.value)}
                disabled={isLoading || isRunning}
              >
                {topology?.nodes.map((node) => (
                  <option key={node.name} value={node.name}>
                    {node.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-[11px] uppercase tracking-[0.16em] text-zinc-500" htmlFor="ghost-horizon">
                Horizon
              </label>
              <div className="mt-2 flex items-center gap-3">
                <input
                  id="ghost-horizon"
                  className="w-full accent-[var(--accent)]"
                  type="range"
                  min={300}
                  max={1800}
                  step={300}
                  value={horizonSeconds}
                  onChange={(event) => setHorizonSeconds(Number(event.target.value))}
                  disabled={isRunning}
                />
                <span className="w-14 text-right text-xs text-zinc-300">{Math.round(horizonSeconds / 60)}m</span>
              </div>
            </div>
            <button
              type="button"
              onClick={() => void runSimulation()}
              disabled={!topology || !selectedNode || isRunning}
              className="btn-primary inline-flex w-full items-center justify-center gap-2"
            >
              <Play size={14} />
              {isRunning ? "Running" : "Simulate Drain"}
            </button>
          </section>

          <section className="surface p-4 space-y-3">
            <p className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">Verdict</p>
            {simulation ? (
              <>
                <div
                  className={`inline-flex items-center gap-2 border px-2 py-1 text-xs ${severityClass(simulation.verdict.severity)}`}
                >
                  <AlertTriangle size={14} />
                  {simulation.verdict.severity}
                </div>
                <p className="text-sm leading-6 text-zinc-200">{simulation.verdict.summary}</p>
                <ul className="space-y-2 text-xs text-zinc-400">
                  {simulation.verdict.recommendations.map((item) => (
                    <li key={item}>{item}</li>
                  ))}
                </ul>
              </>
            ) : (
              <p className="text-sm text-zinc-400">No simulation has run for the current topology.</p>
            )}
          </section>

          <section className="grid grid-cols-3 border border-zinc-700 bg-zinc-900">
            <Metric label="Nodes" value={topology?.nodes.length ?? 0} />
            <Metric label="Moved" value={movedPods} />
            <Metric label="Pending" value={evictedPods} />
          </section>
        </aside>

        <section className="space-y-4">
          <GhostTopologyCanvas topology={topology} frame={activeFrame} selectedNode={selectedNode} />
          {simulation && (
            <div className="surface p-3">
              <div className="flex items-center gap-3">
                <span className="text-xs text-zinc-500">t+{simulation.frames[frameIndex]?.offsetSeconds ?? 0}s</span>
                <input
                  className="w-full accent-[var(--accent)]"
                  type="range"
                  min={0}
                  max={Math.max(0, simulation.frames.length - 1)}
                  value={frameIndex}
                  onChange={(event) => setFrameIndex(Number(event.target.value))}
                />
              </div>
            </div>
          )}
        </section>
      </section>

      <section className="table-shell">
        <div className="table-wrap">
          <table className="min-w-full text-left text-sm">
            <thead className="table-head">
              <tr>
                <th className="px-3 py-2">Resource</th>
                <th className="px-3 py-2">Event</th>
                <th className="px-3 py-2">Severity</th>
                <th className="px-3 py-2">Message</th>
              </tr>
            </thead>
            <tbody>
              {(activeFrame?.events ?? []).map((event) => (
                <tr key={`${event.kind}-${event.resource}-${event.message}`} className="border-t border-zinc-800">
                  <td className="px-3 py-2 font-mono text-xs text-zinc-300">{event.resource}</td>
                  <td className="px-3 py-2 text-zinc-300">{event.kind}</td>
                  <td className="px-3 py-2 text-zinc-400">{event.severity}</td>
                  <td className="px-3 py-2 text-zinc-400">{event.message}</td>
                </tr>
              ))}
              {!activeFrame?.events.length && (
                <tr>
                  <td className="px-3 py-4 text-zinc-500" colSpan={4}>
                    Timeline events appear after a simulation run.
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

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="border-l border-zinc-700 px-3 py-2 first:border-l-0">
      <p className="text-[10px] uppercase tracking-[0.16em] text-zinc-500">{label}</p>
      <p className="mt-1 text-lg font-semibold text-zinc-100">{value}</p>
    </div>
  );
}

function GhostTopologyCanvas({
  topology,
  frame,
  selectedNode,
}: {
  topology: GhostTopology | null;
  frame: GhostTimelineFrame | null;
  selectedNode: string;
}) {
  const hostRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) {
      return;
    }
    host.textContent = "";

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0d0d0d);
    const camera = new THREE.PerspectiveCamera(48, 1, 0.1, 1000);
    camera.position.set(0, 24, 38);
    camera.lookAt(0, 0, 0);

    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    host.appendChild(renderer.domElement);

    scene.add(new THREE.AmbientLight(0xd7fff5, 0.8));
    const light = new THREE.DirectionalLight(0xd7fff5, 1.4);
    light.position.set(12, 24, 18);
    scene.add(light);

    const nodePositions = layoutNodes(topology?.nodes.map((node) => node.name) ?? []);
    const framePods = new Map((frame?.pods ?? []).map((pod) => [pod.id, pod]));
    const topologyPods = topology?.pods ?? [];

    for (const node of topology?.nodes ?? []) {
      const position = nodePositions.get(node.name) ?? new THREE.Vector3();
      const material = new THREE.MeshStandardMaterial({
        color: node.name === selectedNode ? 0x00d4a8 : node.unschedulable ? 0xf59e0b : 0x3b82f6,
        emissive: node.name === selectedNode ? 0x003b31 : 0x050505,
        roughness: 0.45,
      });
      const mesh = new THREE.Mesh(new THREE.SphereGeometry(node.name === selectedNode ? 1.55 : 1.25, 32, 16), material);
      mesh.position.copy(position);
      scene.add(mesh);
    }

    for (const pod of topologyPods) {
      const current = framePods.get(pod.id) ?? pod;
      if (!current.nodeName) {
        continue;
      }
      const base = nodePositions.get(current.nodeName);
      if (!base) {
        continue;
      }
      const offset = podOffset(pod.id);
      const position = new THREE.Vector3(base.x + offset.x, base.y + 2.2 + offset.y, base.z + offset.z);
      const color = current.status === "Pending" ? 0xff4444 : current.nodeName !== pod.nodeName ? 0xf59e0b : 0xa1a1aa;
      const mesh = new THREE.Mesh(
        new THREE.SphereGeometry(0.42, 18, 12),
        new THREE.MeshStandardMaterial({ color, roughness: 0.35 }),
      );
      mesh.position.copy(position);
      scene.add(mesh);
      addLine(scene, base, position, color);
    }

    for (const edge of topology?.edges ?? []) {
      if (!edge.from.startsWith("service:") || !edge.to.startsWith("pod:")) {
        continue;
      }
      const podID = edge.to.replace("pod:", "");
      const pod = topologyPods.find((item) => item.id === podID);
      if (!pod?.nodeName) {
        continue;
      }
      const nodePosition = nodePositions.get(pod.nodeName);
      if (!nodePosition) {
        continue;
      }
      addLine(scene, new THREE.Vector3(0, 0, 0), nodePosition, 0x00d4a8);
    }

    let width = 1;
    let height = 1;
    const resize = () => {
      const rect = host.getBoundingClientRect();
      width = Math.max(1, rect.width);
      height = Math.max(1, rect.height);
      renderer.setSize(width, height, false);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(host);

    let frameID = 0;
    const animate = () => {
      frameID = window.requestAnimationFrame(animate);
      scene.rotation.y += 0.0025;
      renderer.render(scene, camera);
    };
    animate();

    return () => {
      window.cancelAnimationFrame(frameID);
      observer.disconnect();
      renderer.dispose();
      scene.traverse((object: unknown) => {
        if (isDisposableMesh(object)) {
          object.geometry.dispose();
          if (Array.isArray(object.material)) {
            object.material.forEach((material: { dispose: () => void }) => material.dispose());
          } else {
            object.material.dispose();
          }
        }
      });
      host.textContent = "";
    };
  }, [frame, selectedNode, topology]);

  return (
    <div
      ref={hostRef}
      data-testid="ghost-topology-canvas"
      className="h-[420px] min-h-[320px] w-full overflow-hidden border border-zinc-800 bg-zinc-950 md:h-[560px]"
    />
  );
}

function layoutNodes(names: string[]) {
  const positions = new Map<string, VectorLike>();
  const radius = Math.max(7, names.length * 2.8);
  names.forEach((name, index) => {
    const angle = (index / Math.max(1, names.length)) * Math.PI * 2;
    positions.set(name, new THREE.Vector3(Math.cos(angle) * radius, 0, Math.sin(angle) * radius));
  });
  return positions;
}

function addLine(scene: SceneLike, from: VectorLike, to: VectorLike, color: number) {
  const geometry = new THREE.BufferGeometry().setFromPoints([from, to]);
  const material = new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0.45 });
  scene.add(new THREE.Line(geometry, material));
}

function isDisposableMesh(object: unknown): object is DisposableMeshLike {
  return object instanceof THREE.Mesh;
}

function podOffset(id: string) {
  let total = 0;
  for (let index = 0; index < id.length; index += 1) {
    total += id.charCodeAt(index) * (index + 1);
  }
  return {
    x: ((total % 7) - 3) * 0.55,
    y: ((total % 5) - 2) * 0.25,
    z: (((total >> 3) % 7) - 3) * 0.55,
  };
}

function countMovedPods(topology: GhostTopology | null, frame: GhostTimelineFrame | null) {
  if (!topology || !frame) {
    return 0;
  }
  const previous = new Map(topology.pods.map((pod) => [pod.id, pod.nodeName]));
  return frame.pods.filter((pod: GhostFramePod) => pod.nodeName && previous.get(pod.id) !== pod.nodeName).length;
}

function severityClass(severity: string) {
  if (severity === "critical") {
    return "border-red-500/40 bg-red-500/10 text-red-100";
  }
  if (severity === "warning") {
    return "border-amber-500/40 bg-amber-500/10 text-amber-100";
  }
  return "border-[var(--accent)] bg-[var(--accent-dim)] text-zinc-100";
}
