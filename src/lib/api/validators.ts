import type { RAGDocFeedback, RAGQueryTrace, RAGResultTrace, RAGTelemetry } from "../../types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isRAGResultTrace(value: unknown): value is RAGResultTrace {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.title === "string" &&
    typeof value.url === "string" &&
    typeof value.source === "string" &&
    typeof value.finalScore === "number" &&
    typeof value.lexicalScore === "number" &&
    typeof value.semanticScore === "number" &&
    typeof value.coverageScore === "number" &&
    typeof value.sourceBoost === "number" &&
    typeof value.feedbackBoost === "number"
  );
}

function isRAGQueryTrace(value: unknown): value is RAGQueryTrace {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.timestamp === "string" &&
    typeof value.query === "string" &&
    isStringArray(value.queryTerms) &&
    typeof value.usedSemantic === "boolean" &&
    typeof value.candidateCount === "number" &&
    typeof value.resultCount === "number" &&
    typeof value.durationMs === "number" &&
    Array.isArray(value.topResults) &&
    value.topResults.every(isRAGResultTrace)
  );
}

function isRAGDocFeedback(value: unknown): value is RAGDocFeedback {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.url === "string" &&
    typeof value.helpful === "number" &&
    typeof value.notHelpful === "number" &&
    typeof value.netScore === "number" &&
    typeof value.updatedAt === "string"
  );
}

export function isRAGTelemetry(value: unknown): value is RAGTelemetry {
  if (!isRecord(value)) {
    return false;
  }
  return (
    typeof value.enabled === "boolean" &&
    typeof value.indexedAt === "string" &&
    typeof value.expiresAt === "string" &&
    typeof value.totalQueries === "number" &&
    typeof value.emptyResults === "number" &&
    typeof value.hitRate === "number" &&
    typeof value.averageResults === "number" &&
    typeof value.feedbackSignals === "number" &&
    typeof value.positiveFeedback === "number" &&
    typeof value.negativeFeedback === "number" &&
    Array.isArray(value.topFeedbackDocs) &&
    value.topFeedbackDocs.every(isRAGDocFeedback) &&
    Array.isArray(value.recentQueries) &&
    value.recentQueries.every(isRAGQueryTrace)
  );
}
