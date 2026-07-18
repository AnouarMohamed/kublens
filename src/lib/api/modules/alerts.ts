import type {
  AlertDispatchRequest,
  AlertDispatchResponse,
  AuditLogResponse,
  AuditVerification,
  NodeAlertLifecycle,
  NodeAlertLifecycleUpdateRequest,
} from "../../../types";
import { apiRoute, requestJson } from "../core";

export const alertsApi = {
  dispatchAlert: (payload: AlertDispatchRequest) =>
    requestJson<AlertDispatchResponse>(apiRoute("/alerts/dispatch"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  sendTestAlert: () =>
    requestJson<AlertDispatchResponse>(apiRoute("/alerts/test"), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  getAlertLifecycle: (signal?: AbortSignal) =>
    requestJson<NodeAlertLifecycle[]>(apiRoute("/alerts/lifecycle"), { signal }),
  updateAlertLifecycle: (payload: NodeAlertLifecycleUpdateRequest) =>
    requestJson<NodeAlertLifecycle>(apiRoute("/alerts/lifecycle"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getAuditLog: (limit = 120) => requestJson<AuditLogResponse>(`${apiRoute("/audit")}?limit=${limit}`),
  verifyAuditEntry: (id: string) => requestJson<AuditVerification>(apiRoute("/audit/{id}/verify", { id })),
};
