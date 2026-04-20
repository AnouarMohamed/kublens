import { useEffect, useMemo, useState } from "react";
import {
  AlarmClock,
  Archive,
  BookOpen,
  Bot,
  BrainCircuit,
  Boxes,
  Briefcase,
  ClipboardList,
  Clock3,
  Copy,
  Cpu,
  Database,
  FolderTree,
  Globe,
  HardDrive,
  KeyRound,
  Layers,
  LayoutDashboard,
  LineChart,
  Network,
  Rocket,
  ScanSearch,
  ScrollText,
  Server,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Siren,
  SlidersHorizontal,
  TrendingUp,
  Users,
  Wrench,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { VIEW_SECTIONS, type ViewSection } from "../features/viewCatalog";
import { ApiError, api } from "../lib/api";
import type { BuildInfo, ClusterStats, View } from "../types";

interface SidebarProps {
  currentView: View;
  onViewChange: (view: View) => void;
  sections?: ViewSection[];
}

const VIEW_ICON: Record<View, LucideIcon> = {
  overview: LayoutDashboard,
  pods: Boxes,
  deployments: Rocket,
  replicasets: Copy,
  statefulsets: Database,
  daemonsets: Server,
  jobs: Briefcase,
  cronjobs: Clock3,
  services: Network,
  ingresses: Globe,
  networkpolicies: Shield,
  configmaps: SlidersHorizontal,
  secrets: KeyRound,
  persistentvolumes: HardDrive,
  persistentvolumeclaims: Archive,
  storageclasses: Layers,
  nodes: Cpu,
  namespaces: FolderTree,
  events: Zap,
  serviceaccounts: Users,
  rbac: ShieldCheck,
  metrics: LineChart,
  audit: ScrollText,
  predictions: TrendingUp,
  diagnostics: ScanSearch,
  shiftbrief: AlarmClock,
  playbooks: BookOpen,
  incidents: Siren,
  remediation: Wrench,
  memory: BrainCircuit,
  riskguard: ShieldAlert,
  postmortems: ClipboardList,
  assistant: Bot,
};

export default function Sidebar({ currentView, onViewChange, sections = VIEW_SECTIONS }: SidebarProps) {
  const [isReal, setIsReal] = useState(false);
  const [stats, setStats] = useState<ClusterStats | null>(null);
  const [build, setBuild] = useState<BuildInfo | null>(null);
  const [backendLegacy, setBackendLegacy] = useState(false);

  useEffect(() => {
    let cancelled = false;

    Promise.allSettled([api.getClusterInfo(), api.getStats(), api.getVersion()])
      .then(([clusterResult, statsResult, versionResult]) => {
        if (cancelled) {
          return;
        }

        if (clusterResult.status === "fulfilled") {
          setIsReal(clusterResult.value.isRealCluster);
        } else {
          setIsReal(false);
        }

        if (statsResult.status === "fulfilled") {
          setStats(statsResult.value);
        }

        if (versionResult.status === "fulfilled") {
          setBuild(versionResult.value);
          setBackendLegacy(false);
        } else {
          const err = versionResult.reason;
          setBuild(null);
          setBackendLegacy(err instanceof ApiError && err.status === 404);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setIsReal(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const statusSummary = useMemo(() => {
    const podReady = stats?.pods.running ?? 0;
    const podTotal = stats?.pods.total ?? 0;
    const nodeReady = stats?.nodes.ready ?? 0;
    const nodeTotal = stats?.nodes.total ?? 0;
    const readyTotal = podReady + nodeReady;
    const observedTotal = podTotal + nodeTotal;
    const readiness = observedTotal > 0 ? Math.round((readyTotal / observedTotal) * 100) : 0;

    return {
      podReady,
      podTotal,
      nodeReady,
      nodeTotal,
      readyTotal,
      observedTotal,
      readiness,
    };
  }, [stats]);

  return (
    <aside className="w-80 h-screen p-3 pr-2">
      <div className="app-shell h-full flex flex-col overflow-hidden">
        <header className="px-4 py-4 border-b border-zinc-700">
          <p className="text-[10px] uppercase tracking-[0.28em] text-zinc-300">KUBELENS</p>
          <p className="mt-2 text-[11px] text-zinc-500">{isReal ? "cluster: live-runtime" : "cluster: mock-runtime"}</p>

          <div className="mt-3 flex items-center gap-2">
            <span className={`live-dot ${isReal ? "live-dot--active" : ""}`} />
            <span className="text-[11px] text-zinc-400">
              {isReal ? "live connection established" : "mock mode active"}
            </span>
          </div>

          <div className="mt-3 border border-zinc-700 bg-zinc-950 px-3 py-2">
            <div className="flex items-center justify-between text-[10px] uppercase tracking-[0.18em] text-zinc-500">
              <span>
                Pods {statusSummary.podReady}/{statusSummary.podTotal}
              </span>
              <span>
                Nodes {statusSummary.nodeReady}/{statusSummary.nodeTotal}
              </span>
              <span>
                Ready {statusSummary.readyTotal}/{statusSummary.observedTotal}
              </span>
            </div>
            <div className="mt-2 h-1 bg-zinc-800">
              <div className="h-full bg-[var(--accent)]" style={{ width: `${statusSummary.readiness}%` }} />
            </div>
          </div>
        </header>

        <nav className="flex-1 overflow-y-auto px-3 py-3 space-y-4 scrollbar-hide">
          {sections.map((section) => (
            <section key={section.id}>
              <p className="px-2 pb-1 text-[10px] uppercase tracking-[0.24em] text-zinc-500">{section.label}</p>
              <div className="space-y-1">
                {section.items.map((item) => {
                  const active = item.id === currentView;
                  const Icon = VIEW_ICON[item.id];
                  return (
                    <button
                      key={item.id}
                      onClick={() => onViewChange(item.id)}
                      className={`w-full border-l-2 px-2 py-2 text-left ${
                        active ? "border-l-[var(--accent)]" : "border-l-transparent hover:border-l-zinc-600"
                      }`}
                    >
                      <div className="flex items-start gap-2.5">
                        <Icon size={14} className={active ? "text-[var(--accent)]" : "text-zinc-500"} />
                        <div className="min-w-0">
                          <p className="text-sm text-zinc-200 truncate">{item.label}</p>
                          <p className="text-[11px] leading-relaxed text-zinc-500 max-h-8 overflow-hidden prose-text">
                            {item.description}
                          </p>
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
            </section>
          ))}
        </nav>

        <footer className="px-4 py-3 border-t border-zinc-700 bg-zinc-900">
          <p className="text-[10px] uppercase tracking-[0.2em] text-zinc-500">Operator Tip</p>
          <p className="mt-1 text-[11px] text-zinc-400 prose-text">
            Press <span className="font-mono text-zinc-300">/</span> to jump into command search.
          </p>
          <div className="mt-2 border border-zinc-700 bg-zinc-950 px-2 py-1.5">
            <p className="text-[10px] uppercase tracking-[0.2em] text-zinc-500">Backend Build</p>
            {build ? (
              <p className="mt-1 text-[11px] text-zinc-300">
                {build.version} @ {shortCommit(build.commit)}
              </p>
            ) : backendLegacy ? (
              <p className="mt-1 text-[11px] text-[var(--amber)]">legacy backend detected</p>
            ) : (
              <p className="mt-1 text-[11px] text-zinc-500">unavailable</p>
            )}
          </div>
        </footer>
      </div>
    </aside>
  );
}

function shortCommit(commit: string): string {
  const trimmed = commit.trim();
  if (trimmed.length <= 8) {
    return trimmed;
  }
  return trimmed.slice(0, 8);
}
