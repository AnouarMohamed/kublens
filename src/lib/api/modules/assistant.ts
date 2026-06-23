import type { ActionResult, AssistantReferenceFeedbackRequest, AssistantResponse, RAGTelemetry } from "../../../types";
import { apiRoute, requestJson } from "../core";
import { isRAGTelemetry } from "../validators";

export const assistantApi = {
  askAssistant: (message: string, namespace?: string) =>
    requestJson<AssistantResponse>(apiRoute("/assistant"), {
      method: "POST",
      body: JSON.stringify({ message, namespace }),
    }),
  submitAssistantReferenceFeedback: (payload: AssistantReferenceFeedbackRequest) =>
    requestJson<ActionResult>(apiRoute("/assistant/references/feedback"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getRAGTelemetry: (limit = 24, signal?: AbortSignal) =>
    requestJson<RAGTelemetry>(
      `${apiRoute("/rag/telemetry")}?limit=${encodeURIComponent(String(limit))}`,
      { signal },
      isRAGTelemetry,
    ),
};
