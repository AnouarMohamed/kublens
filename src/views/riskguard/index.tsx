import { useMemo, useState } from "react";
import { useAuthSession } from "../../context/AuthSessionContext";
import { api } from "../../lib/api";
import type { RiskCheck, RiskReport } from "../../types";

const DEFAULT_MANIFEST = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-gateway
  namespace: production
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: payment-gateway
    spec:
      containers:
        - name: app
          image: ghcr.io/example/payment-gateway:v1.0.0
`;

export default function RiskGuardView() {
  const { can } = useAuthSession();
  const canRead = can("read");

  const [manifest, setManifest] = useState(DEFAULT_MANIFEST);
  const [report, setReport] = useState<RiskReport | null>(null);
  const [showAllChecks, setShowAllChecks] = useState(false);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const checks = useMemo(() => {
    if (!report) {
      return [];
    }
    if (showAllChecks) {
      return report.checks;
    }
    return report.checks.filter((check) => !check.passed);
  }, [report, showAllChecks]);

  const groupedChecks = useMemo(() => groupChecksByCategory(checks), [checks]);

  const analyze = async () => {
    if (!canRead) {
      return;
    }
    setIsAnalyzing(true);
    try {
      const data = await api.analyzeRiskGuard({ manifest });
      setReport(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Risk analysis failed");
    } finally {
      setIsAnalyzing(false);
    }
  };

  return (
    <div className="space-y-4">
      <header className="panel-head">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100 tracking-tight">Change Risk Guard</h2>
          <p className="text-sm text-zinc-400 mt-1">
            Pre-deploy YAML risk analysis with cluster-aware policy preflight and mitigation guidance.
          </p>
        </div>
        <button onClick={() => void analyze()} disabled={!canRead || isAnalyzing} className="btn-primary">
          {isAnalyzing ? "Analyzing" : "Analyze"}
        </button>
      </header>

      {error && (
        <div className="rounded-md border border-zinc-700 bg-zinc-900/80 px-3 py-2 text-sm text-zinc-200">{error}</div>
      )}

      <section className="surface p-4">
        <label className="text-xs uppercase tracking-wide text-zinc-500">Manifest YAML</label>
        <textarea
          value={manifest}
          onChange={(event) => setManifest(event.target.value)}
          className="field mt-2 w-full min-h-[22rem] font-mono text-xs"
          placeholder="Paste Kubernetes manifest YAML"
        />
      </section>

      {report && (
        <section className="surface p-4">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-wide text-zinc-500">Risk Score</p>
              <p className={`mt-1 text-5xl font-semibold ${scoreColor(report.score)}`}>{report.score}</p>
              <p className="mt-1 text-sm text-zinc-300">{report.level}</p>
              <p className="mt-1 text-sm text-zinc-400">{report.summary}</p>
            </div>
            <button onClick={() => setShowAllChecks((v) => !v)} className="btn">
              {showAllChecks ? "Show failures only" : "Show all checks"}
            </button>
          </div>

          <div className="mt-4 grid gap-3 md:grid-cols-3">
            {groupedChecks.map(([category, items]) => (
              <div key={category} className="border border-zinc-700 bg-zinc-950 px-3 py-2">
                <p className="text-[11px] uppercase tracking-[0.18em] text-zinc-500">{category}</p>
                <p className="mt-1 text-lg font-semibold text-zinc-100">
                  {items.filter((item) => item.passed).length}/{items.length}
                </p>
                <p className="text-xs text-zinc-400">{items.filter((item) => !item.passed).length} flagged checks</p>
              </div>
            ))}
          </div>

          <div className="my-4 border-t border-zinc-700" />

          <div className="space-y-5">
            {groupedChecks.map(([category, items]) => (
              <section key={category}>
                <p className="mb-3 text-[11px] uppercase tracking-[0.22em] text-zinc-500">{category}</p>
                <div className="space-y-3">
                  {items.map((check) => (
                    <details
                      key={`${category}-${check.name}`}
                      open={!check.passed}
                      className="rounded-md border border-zinc-700 bg-zinc-900/70 p-3"
                    >
                      <summary className="cursor-pointer flex items-center justify-between gap-2">
                        <span className="text-sm font-semibold text-zinc-100">
                          {check.passed ? "[pass]" : "[flag]"} {check.name}
                        </span>
                        <span className="text-xs text-zinc-500">Score +{check.score}</span>
                      </summary>
                      <p className="mt-2 text-sm text-zinc-300">{check.detail}</p>
                      <p className="mt-1 text-sm text-zinc-200">
                        <span className="font-semibold">Fix:</span> {check.suggestion}
                      </p>
                    </details>
                  ))}
                </div>
              </section>
            ))}
            {checks.length === 0 && <p className="text-sm text-zinc-500">No failed checks.</p>}
          </div>
        </section>
      )}
    </div>
  );
}

function scoreColor(score: number): string {
  if (score >= 76) {
    return "text-[var(--red)]";
  }
  if (score >= 51) {
    return "text-orange-400";
  }
  if (score >= 26) {
    return "text-[var(--amber)]";
  }
  return "text-[#34c759]";
}

function groupChecksByCategory(checks: RiskCheck[]): Array<[string, RiskCheck[]]> {
  const groups = new Map<string, RiskCheck[]>();
  for (const check of checks) {
    const key = check.category?.trim() || "General";
    const current = groups.get(key) ?? [];
    current.push(check);
    groups.set(key, current);
  }
  return Array.from(groups.entries());
}
