import type { AssistantIntent } from "./constants";

export function applyIntentToPrompt(prompt: string, intent: AssistantIntent): string {
  const trimmed = prompt.trim();
  if (trimmed === "") {
    return trimmed;
  }

  if (intent === "triage") {
    return `Triage mode: prioritize probable root causes, confidence level, and immediate next checks.\n\n${trimmed}`;
  }
  if (intent === "remediate") {
    return `Remediation mode: provide safest fix path first, include rollback plan and risk notes.\n\n${trimmed}`;
  }
  return `Verification mode: provide post-change validation checks, expected signals, and watchouts.\n\n${trimmed}`;
}

export function buildFollowUpPrompt(prefix: string, answer?: string): string {
  const trimmed = (answer ?? "").trim();
  if (trimmed === "") {
    return "Show cluster health";
  }

  const compact = trimmed.length > 1200 ? `${trimmed.slice(0, 1200)}...` : trimmed;
  return `${prefix}:\n\n${compact}`;
}

export function toDiagnosePrompt(resource: string): string {
  const trimmed = resource.trim();
  if (trimmed === "") {
    return "Show cluster health";
  }
  const podName = trimmed.includes("/") ? (trimmed.split("/").pop() ?? trimmed) : trimmed;
  return `Diagnose ${podName}`;
}

export function dedupeStrings(values: readonly string[]): string[] {
  const out: string[] = [];
  for (const value of values) {
    const normalized = value.trim();
    if (normalized === "") {
      continue;
    }
    if (!out.includes(normalized)) {
      out.push(normalized);
    }
  }
  return out;
}
