import type { DiagnosticIssue, DiagnosticsResult } from "../../../types";
import type { AssistantMessage } from "../types";

function formatIssueLine(issue: DiagnosticIssue): string {
  const resource = issue.resource ? ` (${issue.resource})` : "";
  const evidence = (issue.evidence ?? []).join(" | ");
  return `- ${issue.message}${resource}: ${evidence || "no evidence captured yet"}`;
}

function dedupeStrings(values: readonly string[]): string[] {
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

export function buildDiagnosticsIntroMessage(
  diagnostics: DiagnosticsResult,
  createID: () => string,
): AssistantMessage | null {
  const issues = diagnostics.issues ?? [];
  const visibleIssues = issues.filter((issue) => issue.severity !== "info");

  const lines: string[] = [];
  if (visibleIssues.length === 0) {
    lines.push("Diagnostics check is clean. No critical or warning issues detected.");
  } else {
    lines.push(
      `I can see ${diagnostics.criticalIssues} critical and ${diagnostics.warningIssues} warning issues in this cluster.`,
    );
    lines.push("");
    lines.push("Top findings:");
    for (const issue of visibleIssues.slice(0, 3)) {
      lines.push(formatIssueLine(issue));
    }
    lines.push("");
    lines.push("Want me to investigate any of these?");
  }

  const resources = issues
    .map((issue) => issue.resource)
    .filter((resource): resource is string => typeof resource === "string" && resource.trim() !== "");

  const hints = visibleIssues.length
    ? ["What should I fix first?", "Show failed pods", "Show node risks"]
    : ["Show cluster health", "What should I fix first?"];

  return {
    id: createID(),
    role: "assistant",
    content: lines.join("\n"),
    timestamp: new Date().toISOString(),
    hints,
    resources,
  };
}

export function buildDiagnosticPrompts(diagnostics: DiagnosticsResult): string[] {
  const issuePrompts: string[] = [];
  for (const issue of diagnostics.issues.slice(0, 5)) {
    const resourcePrompt = issue.resource?.trim()
      ? `Diagnose ${issue.resource}`
      : `Investigate issue: ${issue.message}`;
    issuePrompts.push(resourcePrompt);
    issuePrompts.push(`How do I safely fix: ${issue.message}`);
  }
  issuePrompts.push("What is the safest next change?");
  issuePrompts.push("Give me a rollback-first runbook.");
  return dedupeStrings(issuePrompts).slice(0, 8);
}
